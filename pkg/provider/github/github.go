package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/go-github/v47/github"
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
	// we can perhaps do some autodetction with event.Provider.GHEURL and adding
	// a raw into it
	publicRawURLHost = "raw.githubusercontent.com"
)

type Provider struct {
	Client        *github.Client
	Logger        *zap.SugaredLogger
	Token, APIURL *string
	ApplicationID *int64
	providerName  string
}

// splitGithubURL Take a Github url and split it with org/repo path ref, supports rawURL
func splitGithubURL(uri string) (string, string, string, string, error) {
	pURL, err := url.Parse(uri)
	splitted := strings.Split(pURL.Path, "/")
	if len(splitted) <= 3 {
		return "", "", "", "", fmt.Errorf("URL %s does not seem to be a proper provider url: %w", uri, err)
	}
	var spOrg, spRepo, spRef, spPath string
	switch {
	case pURL.Host == publicRawURLHost && len(splitted) >= 5:
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
	return spOrg, spRepo, spPath, spRef, nil
}

func (v *Provider) GetTaskURI(ctx context.Context, params *params.Run, event *info.Event, uri string) (bool, string, error) {
	if ret := provider.CompareHostOfURLS(uri, event.URL); !ret {
		return false, "", nil
	}

	spOrg, spRepo, spPath, spRef, err := splitGithubURL(uri)
	if err != nil {
		return false, "", err
	}
	nEvent := info.NewEvent()
	nEvent.Organization = spOrg
	nEvent.Repository = spRepo
	ret, err := v.GetFileInsideRepo(ctx, nEvent, spPath, spRef)
	if err != nil {
		return false, "", err
	}
	return true, ret, nil
}

func (v *Provider) InitAppClient(ctx context.Context, kube kubernetes.Interface, event *info.Event) error {
	var err error
	event.Provider.Token, err = v.getAppToken(ctx, kube, event.GHEURL, event.InstallationID)
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

func (v *Provider) SetClient(ctx context.Context, event *info.Event) error {
	client, providerName, apiURL := makeClient(ctx, event.Provider.URL, event.Provider.Token)
	v.providerName = providerName

	// Make sure Client is not already set, so we don't override our fakeclient
	// from unittesting.
	if v.Client == nil {
		v.Client = client
	}

	v.APIURL = apiURL
	return nil
}

// GetTektonDir Get all yaml files in tekton directory return as a single concated file
func (v *Provider) GetTektonDir(ctx context.Context, runevent *info.Event, path string) (string, error) {
	tektonDirSha := ""

	rootobjects, _, err := v.Client.Git.GetTree(ctx, runevent.Organization, runevent.Repository, runevent.SHA, false)
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
// branch is true, the user the branch as ref isntead of the SHA
// TODO: merge GetFileInsideRepo amd GetTektonDir
func (v *Provider) GetFileInsideRepo(ctx context.Context, runevent *info.Event, path, target string) (string, error) {
	ref := runevent.SHA
	if target != "" {
		ref = runevent.BaseBranch
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
	runevent.EventType = "pull_request"
	return runevent, nil
}

// GetFiles get a files from pull request
func (v *Provider) GetFiles(ctx context.Context, runevent *info.Event) ([]string, error) {
	if runevent.TriggerTarget == "pull_request" {
		repoCommit, _, err := v.Client.PullRequests.ListFiles(ctx, runevent.Organization, runevent.Repository, runevent.PullRequestNumber, &github.ListOptions{})
		if err != nil {
			return []string{}, err
		}
		result := []string{}
		for j := range repoCommit {
			result = append(result, *repoCommit[j].Filename)
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

// Detect processes event and detect if it is a github event, whether to process or reject it
// returns (if is a GH event, whether to process or reject, error if any occurred)
func (v *Provider) Detect(req *http.Request, payload string, logger *zap.SugaredLogger) (bool, bool, *zap.SugaredLogger, string, error) {
	// gitea set x-github-event too, so skip it for the gitea driver
	if h := req.Header.Get("X-Gitea-Event-Type"); h != "" {
		return false, false, logger, "", nil
	}
	isGH := false
	event := req.Header.Get("X-Github-Event")
	if event == "" {
		return false, false, logger, "", nil
	}

	// it is a Github event
	isGH = true

	setLoggerAndProceed := func(processEvent bool, reason string, err error) (bool, bool, *zap.SugaredLogger,
		string, error,
	) {
		logger = logger.With("provider", "github", "event-id", req.Header.Get("X-GitHub-Delivery"))
		return isGH, processEvent, logger, reason, err
	}

	eventInt, err := github.ParseWebHook(event, []byte(payload))
	if err != nil {
		return setLoggerAndProceed(false, "", err)
	}

	_ = json.Unmarshal([]byte(payload), &eventInt)

	switch gitEvent := eventInt.(type) {
	case *github.CheckRunEvent:
		if gitEvent.GetAction() == "rerequested" && gitEvent.GetCheckRun() != nil {
			return setLoggerAndProceed(true, "", nil)
		}
		return setLoggerAndProceed(false, fmt.Sprintf("check_run: unsupported action \"%s\"", gitEvent.GetAction()), nil)

	case *github.IssueCommentEvent:
		if gitEvent.GetAction() == "created" &&
			gitEvent.GetIssue().IsPullRequest() &&
			gitEvent.GetIssue().GetState() == "open" {
			if provider.IsTestRetestComment(gitEvent.GetComment().GetBody()) {
				return setLoggerAndProceed(true, "", nil)
			}
			if provider.IsOkToTestComment(gitEvent.GetComment().GetBody()) {
				return setLoggerAndProceed(true, "", nil)
			}
			return setLoggerAndProceed(false, "", nil)
		}
		return setLoggerAndProceed(false, "issue: not a gitops pull request comment", nil)
	case *github.PushEvent:
		if gitEvent.GetPusher() != nil {
			return setLoggerAndProceed(true, "", nil)
		}
		return setLoggerAndProceed(false, "push: no pusher in event", nil)

	case *github.PullRequestEvent:
		if provider.Valid(gitEvent.GetAction(), []string{"opened", "synchronize", "synchronized", "reopened"}) {
			return setLoggerAndProceed(true, "", nil)
		}
		return setLoggerAndProceed(false, fmt.Sprintf("pull_request: unsupported action \"%s\"", gitEvent.GetAction()), nil)

	default:
		return setLoggerAndProceed(false, fmt.Sprintf("github: event \"%v\" is not supported", event), nil)
	}
}
