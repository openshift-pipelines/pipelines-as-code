package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/google/go-github/v43/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

const apiPublicURL = "https://api.github.com/"

type Provider struct {
	Client        *github.Client
	Logger        *zap.SugaredLogger
	Token, APIURL *string
	ApplicationID *int64

	CheckRunIDS *sync.Map
}

func (v *Provider) SetLogger(logger *zap.SugaredLogger) {
	v.Logger = logger
}

func (v *Provider) Validate(ctx context.Context, cs *params.Run, event *info.Event) error {
	signature := event.Request.Header.Get(github.SHA256SignatureHeader)
	if signature == "" {
		signature = event.Request.Header.Get(github.SHA1SignatureHeader)
	}
	return github.ValidateSignature(signature, event.Request.Payload, []byte(event.Provider.WebhookSecret))
}

func (v *Provider) GetConfig() *info.ProviderConfig {
	return &info.ProviderConfig{
		TaskStatusTMPL: taskStatusTemplate,
		APIURL:         apiPublicURL,
	}
}

func (v *Provider) SetClient(ctx context.Context, event *info.Event) error {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: event.Provider.Token},
	)
	tc := oauth2.NewClient(ctx, ts)
	var client *github.Client
	apiURL := event.Provider.URL
	if apiURL != "" {
		if !strings.HasPrefix(apiURL, "https") {
			apiURL = "https://" + apiURL
		}
	}
	if apiURL != "" && apiURL != apiPublicURL {
		client, _ = github.NewEnterpriseClient(apiURL, apiURL, tc)
	} else {
		client = github.NewClient(tc)
		apiURL = client.BaseURL.String()
	}

	// Make sure Client is not already set, so we don't override our fakeclient
	// from unittesting.
	if v.Client == nil {
		v.Client = client
	}
	v.CheckRunIDS = &sync.Map{}

	v.APIURL = &apiURL

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

	commit, _, err := v.Client.Git.GetCommit(ctx, runevent.Organization, runevent.Repository, runevent.SHA)
	if err != nil {
		return err
	}

	runevent.SHAURL = commit.GetHTMLURL()
	runevent.SHATitle = strings.Split(commit.GetMessage(), "\n\n")[0]

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

	// TODO: check if we really need this
	if runevent.Sender == "" {
		runevent.Sender = pr.GetUser().GetLogin()
	}
	runevent.HeadBranch = pr.GetHead().GetRef()
	runevent.BaseBranch = pr.GetBase().GetRef()
	runevent.EventType = "pull_request"
	return runevent, nil
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
func (v *Provider) Detect(reqHeader *http.Header, payload string, logger *zap.SugaredLogger) (bool, bool, *zap.SugaredLogger, error) {
	isGH := false
	event := reqHeader.Get("X-Github-Event")
	if event == "" {
		return false, false, logger, nil
	}

	// it is a Github event
	isGH = true

	setLoggerAndProceed := func(processEvent bool, err error) (bool, bool, *zap.SugaredLogger, error) {
		logger = logger.With("provider", "github", "event", reqHeader.Get("X-GitHub-Delivery"))
		return isGH, processEvent, logger, err
	}

	eventInt, err := github.ParseWebHook(event, []byte(payload))
	if err != nil {
		return setLoggerAndProceed(false, err)
	}

	_ = json.Unmarshal([]byte(payload), &eventInt)

	switch gitEvent := eventInt.(type) {
	case *github.CheckRunEvent:
		if gitEvent.GetAction() == "rerequested" && gitEvent.GetCheckRun() != nil {
			return setLoggerAndProceed(true, nil)
		}
		return setLoggerAndProceed(false, nil)

	case *github.IssueCommentEvent:
		if gitEvent.GetAction() == "created" &&
			gitEvent.GetIssue().IsPullRequest() &&
			gitEvent.GetIssue().GetState() == "open" {
			if provider.IsRetestComment(gitEvent.GetComment().GetBody()) {
				return setLoggerAndProceed(true, nil)
			}
			if provider.IsOkToTestComment(gitEvent.GetComment().GetBody()) {
				return setLoggerAndProceed(true, nil)
			}
			return setLoggerAndProceed(false, nil)
		}
		return setLoggerAndProceed(false, nil)
	case *github.PushEvent:
		if gitEvent.GetPusher() != nil {
			return setLoggerAndProceed(true, nil)
		}
		return setLoggerAndProceed(false, nil)

	case *github.PullRequestEvent:
		if provider.Valid(gitEvent.GetAction(), []string{"opened", "synchronize", "reopened"}) {
			return setLoggerAndProceed(true, nil)
		}
		return setLoggerAndProceed(false, nil)

	default:
		return setLoggerAndProceed(false, fmt.Errorf("github: event %v is not supported", event))
	}
}
