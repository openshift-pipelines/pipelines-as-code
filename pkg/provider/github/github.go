package github

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v53/github"
	"github.com/jonboulle/clockwork"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"k8s.io/client-go/kubernetes"
)

const (
	apiPublicURL = "https://api.github.com/"
	// TODO: makes this configurable for GHE in the ConfigMap.
	// on our GHE instance, it looks like this :
	// https://raw.ghe.openshiftpipelines.com/pac/chmouel-test/main/README.md
	// we can perhaps do some autodetection with event.Provider.GHEURL and adding
	// a raw into it
	publicRawURLHost = "raw.githubusercontent.com"

	defaultPaginedNumber = 100
)

var _ provider.Interface = (*Provider)(nil)

type Provider struct {
	Client        *github.Client
	Logger        *zap.SugaredLogger
	Token, APIURL *string
	ApplicationID *int64
	providerName  string
	provenance    string
	Run           *params.Run
	RepositoryIDs []int64
	repoSettings  *v1alpha1.Settings
	paginedNumber int
	skippedRun
}

type skippedRun struct {
	mutex      *sync.Mutex
	checkRunID int64
}

func New() *Provider {
	return &Provider{
		APIURL:        github.String(keys.PublicGithubAPIURL),
		paginedNumber: defaultPaginedNumber,
		skippedRun: skippedRun{
			mutex: &sync.Mutex{},
		},
	}
}

// detectGHERawURL Detect if we have a raw URL in GHE
func detectGHERawURL(event *info.Event, taskHost string) bool {
	gheURL, err := url.Parse(event.GHEURL)
	if err != nil {
		// should not happen but may as well make sure
		return false
	}
	return taskHost == fmt.Sprintf("raw.%s", gheURL.Host)
}

// splitGithubURL Take a Github url and split it with org/repo path ref, supports rawURL
func splitGithubURL(event *info.Event, uri string) (string, string, string, string, error) {
	pURL, err := url.Parse(uri)
	if err != nil {
		return "", "", "", "", fmt.Errorf("URL %s does not seem to be a proper provider url: %w", uri, err)
	}
	path := pURL.Path
	if pURL.RawPath != "" {
		path = pURL.RawPath
	}
	splitted := strings.Split(path, "/")
	if len(splitted) <= 3 {
		return "", "", "", "", fmt.Errorf("URL %s does not seem to be a proper provider url: %w", uri, err)
	}
	var spOrg, spRepo, spRef, spPath string
	switch {
	case (pURL.Host == publicRawURLHost || detectGHERawURL(event, pURL.Host)) && len(splitted) >= 5:
		spOrg = splitted[1]
		spRepo = splitted[2]
		spRef = splitted[3]
		spPath = strings.Join(splitted[4:], "/")
	case splitted[3] == "blob" && len(splitted) >= 5:
		spOrg = splitted[1]
		spRepo = splitted[2]
		spRef = splitted[4]
		spPath = strings.Join(splitted[5:], "/")
	default:
		return "", "", "", "", fmt.Errorf("cannot recognize task as a Github URL to fetch: %s", uri)
	}
	// url decode the org, repo, ref and path
	if spRef, err = url.QueryUnescape(spRef); err != nil {
		return "", "", "", "", fmt.Errorf("cannot decode ref: %w", err)
	}
	if spPath, err = url.QueryUnescape(spPath); err != nil {
		return "", "", "", "", fmt.Errorf("cannot decode path: %w", err)
	}
	if spOrg, err = url.QueryUnescape(spOrg); err != nil {
		return "", "", "", "", fmt.Errorf("cannot decode org: %w", err)
	}
	if spRepo, err = url.QueryUnescape(spRepo); err != nil {
		return "", "", "", "", fmt.Errorf("cannot decode repo: %w", err)
	}
	return spOrg, spRepo, spPath, spRef, nil
}

func (v *Provider) GetTaskURI(ctx context.Context, _ *params.Run, event *info.Event, uri string) (bool, string, error) {
	if ret := provider.CompareHostOfURLS(uri, event.URL); !ret {
		return false, "", nil
	}

	spOrg, spRepo, spPath, spRef, err := splitGithubURL(event, uri)
	if err != nil {
		return false, "", err
	}
	nEvent := info.NewEvent()
	nEvent.Organization = spOrg
	nEvent.Repository = spRepo
	nEvent.BaseBranch = spRef
	ret, err := v.GetFileInsideRepo(ctx, nEvent, spPath, spRef)
	if err != nil {
		return false, "", err
	}
	return true, ret, nil
}

func (v *Provider) InitAppClient(ctx context.Context, kube kubernetes.Interface, event *info.Event) error {
	var err error
	// TODO: move this out of here when we move al config inside context
	ns := os.Getenv("SYSTEM_NAMESPACE")
	event.Provider.Token, err = v.GetAppToken(ctx, kube, event.GHEURL, event.InstallationID, ns)
	if err != nil {
		return err
	}

	return nil
}

func (v *Provider) SetLogger(logger *zap.SugaredLogger) {
	v.Logger = logger
}

func (v *Provider) Validate(_ context.Context, _ *params.Run, event *info.Event) error {
	signature := event.Request.Header.Get(github.SHA256SignatureHeader)

	if signature == "" {
		signature = event.Request.Header.Get(github.SHA1SignatureHeader)
	}
	if signature == "" || signature == "sha1=" {
		// if no signature is present then don't validate, because user hasn't set one
		return fmt.Errorf("no signature has been detected, for security reason we are not allowing webhooks that has no secret")
	}
	if event.Provider.WebhookSecret == "" {
		return fmt.Errorf("no webhook secret has been set, in repository CR or secret")
	}
	return github.ValidateSignature(signature, event.Request.Payload, []byte(event.Provider.WebhookSecret))
}

func (v *Provider) GetConfig() *info.ProviderConfig {
	return &info.ProviderConfig{
		TaskStatusTMPL: taskStatusTemplate,
		APIURL:         apiPublicURL,
		Name:           v.providerName,
	}
}

func makeClient(ctx context.Context, apiURL, token string) (*github.Client, string, *string) {
	var client *github.Client
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)

	tc := oauth2.NewClient(ctx, ts)
	if apiURL != "" {
		if !strings.HasPrefix(apiURL, "https") && !strings.HasPrefix(apiURL, "http") {
			apiURL = "https://" + apiURL
		}
	}

	providerName := "github"
	if apiURL != "" && apiURL != apiPublicURL {
		providerName = "github-enterprise"
		client, _ = github.NewEnterpriseClient(apiURL, apiURL, tc)
	} else {
		client = github.NewClient(tc)
		apiURL = client.BaseURL.String()
	}

	return client, providerName, github.String(apiURL)
}

func parseTS(headerTS string) (time.Time, error) {
	ts := time.Time{}
	// Normal UTC: 2023-01-31 23:00:00 UTC
	if t, err := time.Parse("2006-01-02 15:04:05 MST", headerTS); err == nil {
		ts = t
	}

	// With TZ(???), ie: a token from Christoph 2023-04-26 23:23:26 +2000
	if t, err := time.Parse("2006-01-02 15:04:05 -0700", headerTS); err == nil {
		ts = t
	}
	if ts.Year() == 1 {
		return ts, fmt.Errorf("cannot parse token expiration date: %s", headerTS)
	}

	return ts, nil
}

// checkWebhookSecretValidity check the webhook secret is valid and not
// ratelimited. we try to check first the header is set (unlimited life token  would
// not have an expiration) we would anyway get a 401 error when trying to use it
// but this gives a nice hint to the user into their namespace event of where
// the issue was
func (v *Provider) checkWebhookSecretValidity(ctx context.Context, cw clockwork.Clock) error {
	rl, resp, err := v.Client.RateLimits(ctx)
	if resp.Header.Get("GitHub-Authentication-Token-Expiration") != "" {
		ts, err := parseTS(resp.Header.Get("GitHub-Authentication-Token-Expiration"))
		if err != nil {
			return fmt.Errorf("error parsing token expiration date: %w", err)
		}

		if cw.Now().After(ts) {
			errm := fmt.Sprintf("token has expired at %s", resp.TokenExpiration.Format(time.RFC1123))
			return fmt.Errorf(errm)
		}
	}

	if resp.StatusCode == http.StatusNotFound {
		v.Logger.Info("skipping checking if token has expired, rate_limit api is not enabled on token")
		return nil
	}

	// some other error happened that is not rate limited related
	if err != nil {
		return fmt.Errorf("error using token to access API: %w", err)
	}

	if rl.SCIM.Remaining == 0 {
		return fmt.Errorf("token is ratelimited, it will be available again at %s", rl.SCIM.Reset.Format(time.RFC1123))
	}
	return nil
}

func (v *Provider) SetClient(ctx context.Context, run *params.Run, event *info.Event, repoSettings *v1alpha1.Settings) error {
	client, providerName, apiURL := makeClient(ctx, event.Provider.URL, event.Provider.Token)
	v.providerName = providerName
	v.Run = run
	v.repoSettings = repoSettings

	// check that the Client is not already set, so we don't override our fakeclient
	// from unittesting.
	if v.Client == nil {
		v.Client = client
	}

	v.APIURL = apiURL

	if event.Provider.WebhookSecretFromRepo {
		// check the webhook secret is valid and not ratelimited
		if err := v.checkWebhookSecretValidity(ctx, clockwork.NewRealClock()); err != nil {
			return fmt.Errorf("the webhook secret is not valid: %w", err)
		}
	}

	return nil
}

// GetTektonDir Get all yaml files in tekton directory return as a single concated file
func (v *Provider) GetTektonDir(ctx context.Context, runevent *info.Event, path, provenance string) (string, error) {
	tektonDirSha := ""

	v.provenance = provenance
	// default set provenance from the SHA
	revision := runevent.SHA
	if provenance == "default_branch" {
		revision = runevent.DefaultBranch
		v.Logger.Infof("Using PipelineRun definition from default_branch: %s", runevent.DefaultBranch)
	} else {
		v.Logger.Infof("Using PipelineRun definition from source pull request SHA: %s", runevent.SHA)
	}

	rootobjects, _, err := v.Client.Git.GetTree(ctx, runevent.Organization, runevent.Repository, revision, false)
	if err != nil {
		return "", err
	}
	for _, object := range rootobjects.Entries {
		if object.GetPath() == path {
			if object.GetType() != "tree" {
				return "", fmt.Errorf("%s has been found but is not a directory", path)
			}
			tektonDirSha = object.GetSHA()
		}
	}

	// If we didn't find a .tekton directory then just silently ignore the error.
	if tektonDirSha == "" {
		return "", nil
	}

	// Get all files in the .tekton directory recursively
	// there is a limit on this recursive calls to 500 entries, as documented here:
	// https://docs.github.com/en/rest/reference/git#get-a-tree
	// so we may need to address it in the future.
	tektonDirObjects, _, err := v.Client.Git.GetTree(ctx, runevent.Organization, runevent.Repository, tektonDirSha,
		true)
	if err != nil {
		return "", err
	}
	return v.concatAllYamlFiles(ctx, tektonDirObjects.Entries, runevent)
}

// GetCommitInfo get info (url and title) on a commit in runevent, this needs to
// be run after sewebhook while we already matched a token.
func (v *Provider) GetCommitInfo(ctx context.Context, runevent *info.Event) error {
	if v.Client == nil {
		return fmt.Errorf("no github client has been initiliazed, " +
			"exiting... (hint: did you forget setting a secret on your repo?)")
	}

	// if we don't have a sha we may have a branch (ie: incoming webhook) then
	// use the branch as sha since github supports it
	var commit *github.Commit
	sha := runevent.SHA
	if runevent.SHA == "" && runevent.HeadBranch != "" {
		branchinfo, _, err := v.Client.Repositories.GetBranch(ctx, runevent.Organization, runevent.Repository, runevent.HeadBranch, true)
		if err != nil {
			return err
		}
		sha = branchinfo.Commit.GetSHA()
	}
	var err error
	commit, _, err = v.Client.Git.GetCommit(ctx, runevent.Organization, runevent.Repository, sha)
	if err != nil {
		return err
	}

	runevent.SHAURL = commit.GetHTMLURL()
	runevent.SHATitle = strings.Split(commit.GetMessage(), "\n\n")[0]
	runevent.SHA = commit.GetSHA()

	return nil
}

// GetFileInsideRepo Get a file via Github API using the runinfo information, we
// branch is true, the user the branch as ref instead of the SHA
// TODO: merge GetFileInsideRepo amd GetTektonDir
func (v *Provider) GetFileInsideRepo(ctx context.Context, runevent *info.Event, path, target string) (string, error) {
	ref := runevent.SHA
	if target != "" {
		ref = runevent.BaseBranch
	} else if v.provenance == "default_branch" {
		ref = runevent.DefaultBranch
	}

	fp, objects, _, err := v.Client.Repositories.GetContents(ctx, runevent.Organization,
		runevent.Repository, path, &github.RepositoryContentGetOptions{Ref: ref})
	if err != nil {
		return "", err
	}
	if objects != nil {
		return "", fmt.Errorf("referenced file inside the Github Repository %s is a directory", path)
	}

	getobj, err := v.getObject(ctx, fp.GetSHA(), runevent)
	if err != nil {
		return "", err
	}

	return string(getobj), nil
}

// concatAllYamlFiles concat all yaml files from a directory as one big multi document yaml string
func (v *Provider) concatAllYamlFiles(ctx context.Context, objects []*github.TreeEntry, runevent *info.Event) (string, error) {
	var allTemplates string

	for _, value := range objects {
		if strings.HasSuffix(value.GetPath(), ".yaml") ||
			strings.HasSuffix(value.GetPath(), ".yml") {
			data, err := v.getObject(ctx, value.GetSHA(), runevent)
			if err != nil {
				return "", err
			}
			if allTemplates != "" && !strings.HasPrefix(string(data), "---") {
				allTemplates += "---"
			}
			allTemplates += "\n" + string(data) + "\n"
		}
	}
	return allTemplates, nil
}

// getPullRequest get a pull request details
func (v *Provider) getPullRequest(ctx context.Context, runevent *info.Event) (*info.Event, error) {
	pr, _, err := v.Client.PullRequests.Get(ctx, runevent.Organization, runevent.Repository, runevent.PullRequestNumber)
	if err != nil {
		return runevent, err
	}
	// Make sure to use the Base for Default BaseBranch or there would be a potential hijack
	runevent.DefaultBranch = pr.GetBase().GetRepo().GetDefaultBranch()
	runevent.URL = pr.GetBase().GetRepo().GetHTMLURL()
	runevent.SHA = pr.GetHead().GetSHA()
	runevent.SHAURL = fmt.Sprintf("%s/commit/%s", pr.GetHTMLURL(), pr.GetHead().GetSHA())
	runevent.PullRequestTitle = pr.GetTitle()

	// TODO: check if we really need this
	if runevent.Sender == "" {
		runevent.Sender = pr.GetUser().GetLogin()
	}
	runevent.HeadBranch = pr.GetHead().GetRef()
	runevent.BaseBranch = pr.GetBase().GetRef()
	runevent.HeadURL = pr.GetHead().GetRepo().GetHTMLURL()
	runevent.BaseURL = pr.GetBase().GetRepo().GetHTMLURL()
	runevent.EventType = "pull_request"

	v.RepositoryIDs = []int64{
		pr.GetBase().GetRepo().GetID(),
	}
	return runevent, nil
}

// GetFiles get a files from pull request
func (v *Provider) GetFiles(ctx context.Context, runevent *info.Event) ([]string, error) {
	if runevent.TriggerTarget == "pull_request" {
		opt := &github.ListOptions{PerPage: v.paginedNumber}
		result := []string{}
		for {
			repoCommit, resp, err := v.Client.PullRequests.ListFiles(ctx, runevent.Organization, runevent.Repository, runevent.PullRequestNumber, opt)
			if err != nil {
				return []string{}, err
			}
			for j := range repoCommit {
				result = append(result, *repoCommit[j].Filename)
			}
			if resp.NextPage == 0 {
				break
			}
			opt.Page = resp.NextPage
		}
		return result, nil
	}

	if runevent.TriggerTarget == "push" {
		result := []string{}
		rC, _, err := v.Client.Repositories.GetCommit(ctx, runevent.Organization, runevent.Repository, runevent.SHA, &github.ListOptions{})
		if err != nil {
			return []string{}, err
		}
		for i := range rC.Files {
			result = append(result, *rC.Files[i].Filename)
		}
		return result, nil
	}
	return []string{}, nil
}

// getObject Get an object from a repository
func (v *Provider) getObject(ctx context.Context, sha string, runevent *info.Event) ([]byte, error) {
	blob, _, err := v.Client.Git.GetBlob(ctx, runevent.Organization, runevent.Repository, sha)
	if err != nil {
		return nil, err
	}

	decoded, err := base64.StdEncoding.DecodeString(blob.GetContent())
	if err != nil {
		return nil, err
	}
	return decoded, err
}

// ListRepos lists all the repos for a particular token
func ListRepos(ctx context.Context, v *Provider) ([]string, error) {
	if v.Client == nil {
		return []string{}, fmt.Errorf("no github client has been initiliazed, " +
			"exiting... (hint: did you forget setting a secret on your repo?)")
	}

	opt := &github.ListOptions{PerPage: v.paginedNumber}
	repoURLs := []string{}
	for {
		repoList, resp, err := v.Client.Apps.ListRepos(ctx, opt)
		if err != nil {
			return []string{}, err
		}
		for i := range repoList.Repositories {
			repoURLs = append(repoURLs, *repoList.Repositories[i].HTMLURL)
		}
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return repoURLs, nil
}

func (v *Provider) CreateToken(ctx context.Context, repository []string, run *params.Run, event *info.Event) (string, error) {
	for _, r := range repository {
		split := strings.Split(r, "/")
		infoData, _, err := v.Client.Repositories.Get(ctx, split[0], split[1])
		if err != nil {
			v.Logger.Warn("we have an invalid repository: `%s` or no access to it: %v", r, err)
			continue
		}
		v.RepositoryIDs = uniqueRepositoryID(v.RepositoryIDs, infoData.GetID())
	}
	token, err := v.GetAppToken(ctx, run.Clients.Kube, event.Provider.URL, event.InstallationID, os.Getenv("SYSTEM_NAMESPACE"))
	if err != nil {
		return "", err
	}
	return token, nil
}

func uniqueRepositoryID(repoIDs []int64, id int64) []int64 {
	r := repoIDs
	m := make(map[int64]bool)
	for _, val := range repoIDs {
		if _, ok := m[val]; !ok {
			m[val] = true
		}
	}
	if _, ok := m[id]; !ok {
		r = append(r, id)
	}
	return r
}
