package gitea

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"strconv"
	"strings"

	giteastruct "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/sdk/gitea"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"go.uber.org/zap"
)

const (
	taskStatusTemplate = `
<table>
  <tr><th>Status</th><th>Duration</th><th>Name</th></tr>

{{- range $taskrun := .TaskRunList }}
<tr>
<td>{{ formatCondition $taskrun.Status.Conditions }}</td>
<td>{{ formatDuration $taskrun.Status.StartTime $taskrun.Status.CompletionTime }}</td><td>

{{ $taskrun.ConsoleLogURL }}

</td></tr>
{{- end }}
</table>`
)

type Provider struct {
	Client           *gitea.Client
	Logger           *zap.SugaredLogger
	Token            *string
	giteaInstanceURL string
	// only exposed for e2e tests
	Password string
}

func (v *Provider) GetTaskURI(ctx context.Context, params *params.Run, event *info.Event, uri string) (bool, string, error) {
	// TODO implement me
	panic("implement me")
}

func (v *Provider) SetLogger(logger *zap.SugaredLogger) {
	v.Logger = logger
}

func (v *Provider) Validate(_ context.Context, _ *params.Run, event *info.Event) error {
	// TODO: figure out why gitea doesn't work with mac validation as github which seems to be the same
	v.Logger.Debug("no secret and signature found, skipping validation for gitea")
	return nil
}

// Detect processes event and detect if it is a gitea event, whether to process or reject it
// returns (if is a Gitea event, whether to process or reject, logger with event metadata,, error if any occurred)
func (v *Provider) Detect(req *http.Request, payload string, logger *zap.SugaredLogger) (bool, bool, *zap.SugaredLogger, string, error) {
	isGitea := false
	event := req.Header.Get("X-Gitea-Event-Type")
	if event == "" {
		return false, false, logger, "no gitea event", nil
	}

	isGitea = true
	setLoggerAndProceed := func(processEvent bool, reason string, err error) (bool, bool, *zap.SugaredLogger,
		string, error,
	) {
		logger = logger.With("provider", "gitea", "event-id", req.Header.Get("X-Request-Id"))
		return isGitea, processEvent, logger, reason, err
	}

	eventInt, err := parseWebhook(whEventType(event), []byte(payload))
	if err != nil {
		return setLoggerAndProceed(false, "", err)
	}
	_ = json.Unmarshal([]byte(payload), &eventInt)

	switch gitEvent := eventInt.(type) {
	case *giteastruct.IssueCommentPayload:
		if gitEvent.Action == "created" &&
			gitEvent.Issue.PullRequest != nil &&
			gitEvent.Issue.State == "open" {
			if provider.IsTestRetestComment(gitEvent.Comment.Body) {
				return setLoggerAndProceed(true, "", nil)
			}
			if provider.IsOkToTestComment(gitEvent.Comment.Body) {
				return setLoggerAndProceed(true, "", nil)
			}
			return setLoggerAndProceed(false, "", nil)
		}
		return setLoggerAndProceed(false, "not a issue comment we care about", nil)
	case *giteastruct.PullRequestPayload:
		if provider.Valid(string(gitEvent.Action), []string{"opened", "synchronize", "synchronized", "reopened"}) {
			return setLoggerAndProceed(true, "", nil)
		}
		return setLoggerAndProceed(false, fmt.Sprintf("not a merge event we care about: \"%s\"",
			string(gitEvent.Action)), nil)
	case *giteastruct.PushPayload:
		if gitEvent.Pusher != nil {
			return setLoggerAndProceed(true, "", nil)
		}
		return setLoggerAndProceed(false, "push: no pusher in event", nil)
	default:
		return setLoggerAndProceed(false, "", fmt.Errorf("gitea: event \"%s\" is not supported", event))
	}
}

func (v *Provider) ParsePayload(_ context.Context, _ *params.Run, request *http.Request,
	payload string,
) (*info.Event, error) {
	// TODO: parse request to figure out which event
	var processedEvent *info.Event

	eventType := request.Header.Get("X-Gitea-Event-Type")
	if eventType == "" {
		return nil, fmt.Errorf("failed to find event type in request header")
	}

	payloadB := []byte(payload)
	eventInt, err := parseWebhook(whEventType(eventType), payloadB)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(payloadB, &eventInt)

	switch gitEvent := eventInt.(type) {
	case *giteastruct.PullRequestPayload:
		processedEvent = info.NewEvent()
		// // Organization:  event.GetRepo().GetOwner().GetLogin(),
		processedEvent.Sender = gitEvent.Sender.UserName
		processedEvent.DefaultBranch = gitEvent.Repository.DefaultBranch
		processedEvent.URL = gitEvent.Repository.HTMLURL
		processedEvent.SHA = gitEvent.PullRequest.Head.Sha
		processedEvent.SHAURL = fmt.Sprintf("%s/commit/%s", gitEvent.PullRequest.HTMLURL, processedEvent.SHA)
		processedEvent.HeadBranch = gitEvent.PullRequest.Head.Ref
		processedEvent.BaseBranch = gitEvent.PullRequest.Base.Ref
		processedEvent.PullRequestNumber = int(gitEvent.Index)
		processedEvent.PullRequestTitle = gitEvent.PullRequest.Title
		processedEvent.Organization = gitEvent.Repository.Owner.UserName
		processedEvent.Repository = gitEvent.Repository.Name
		processedEvent.TriggerTarget = "pull_request"
		processedEvent.EventType = "pull_request"
	case *giteastruct.PushPayload:
		if len(gitEvent.Commits) == 0 {
			return nil, fmt.Errorf("no commits attached to this push event")
		}
		processedEvent = info.NewEvent()
		processedEvent.Organization = gitEvent.Repo.Owner.UserName
		processedEvent.Repository = gitEvent.Repo.Name
		processedEvent.DefaultBranch = gitEvent.Repo.DefaultBranch
		processedEvent.URL = gitEvent.Repo.HTMLURL
		processedEvent.SHA = gitEvent.HeadCommit.ID
		if processedEvent.SHA == "" {
			processedEvent.SHA = gitEvent.Before
		}
		processedEvent.Sender = gitEvent.Sender.UserName
		processedEvent.SHAURL = gitEvent.HeadCommit.URL
		processedEvent.SHATitle = gitEvent.HeadCommit.Message
		processedEvent.BaseBranch = gitEvent.Ref
		processedEvent.EventType = eventType
		processedEvent.HeadBranch = processedEvent.BaseBranch // in push events Head Branch is the same as Basebranch
		processedEvent.TriggerTarget = "push"
	case *giteastruct.IssueCommentPayload:
		if gitEvent.Issue.PullRequest == nil {
			return info.NewEvent(), fmt.Errorf("issue comment is not coming from a pull_request")
		}
		processedEvent = info.NewEvent()
		processedEvent.Organization = gitEvent.Repository.Owner.UserName
		processedEvent.Repository = gitEvent.Repository.Name
		processedEvent.Sender = gitEvent.Sender.UserName
		processedEvent.TriggerTarget = "pull_request"
		processedEvent.EventType = "pull_request"

		if provider.IsTestRetestComment(gitEvent.Comment.Body) {
			processedEvent.TargetTestPipelineRun = provider.GetPipelineRunFromComment(gitEvent.Comment.Body)
		}
		processedEvent.PullRequestNumber, err = convertPullRequestURLtoNumber(gitEvent.Issue.URL)
		if err != nil {
			return nil, err
		}
		processedEvent.URL = gitEvent.Repository.HTMLURL
		processedEvent.DefaultBranch = gitEvent.Repository.DefaultBranch
	default:
		return nil, fmt.Errorf("event %s is not supported", eventType)
	}

	processedEvent.Event = eventInt
	return processedEvent, nil
}

func convertPullRequestURLtoNumber(pullRequest string) (int, error) {
	prNumber, err := strconv.Atoi(path.Base(pullRequest))
	if err != nil {
		return -1, fmt.Errorf("bad pull request number html_url number: %w", err)
	}
	return prNumber, nil
}

func (v *Provider) GetConfig() *info.ProviderConfig {
	return &info.ProviderConfig{
		TaskStatusTMPL: taskStatusTemplate,
		APIURL:         v.giteaInstanceURL,
		Name:           "gitea",
		SkipEmoji:      true,
	}
}

func (v *Provider) SetClient(_ context.Context, runevent *info.Event) error {
	var err error
	apiURL := runevent.Provider.URL
	// password is not exposed to CRD, it's only used from the e2e tests
	if v.Password != "" && runevent.Provider.User != "" {
		v.Client, err = gitea.NewClient(apiURL, gitea.SetBasicAuth(runevent.Provider.User, v.Password))
	} else {
		if runevent.Provider.Token == "" {
			return fmt.Errorf("no git_provider.secret has been set in the repo crd")
		}
		v.Client, err = gitea.NewClient(apiURL, gitea.SetToken(runevent.Provider.Token))
	}
	if err != nil {
		return err
	}
	v.giteaInstanceURL = runevent.Provider.URL
	return nil
}

func (v *Provider) CreateStatus(_ context.Context, _ versioned.Interface, event *info.Event, pacOpts *info.PacOpts,
	statusOpts provider.StatusOpts,
) error {
	if v.Client == nil {
		return fmt.Errorf("cannot set status on gitea no token or url set")
	}
	switch statusOpts.Conclusion {
	case "success":
		statusOpts.Title = "Success"
		statusOpts.Summary = "has <b>successfully</b> validated your commit."
	case "failure":
		statusOpts.Title = "Failed"
		statusOpts.Summary = "has <b>failed</b>."
	case "skipped":
		statusOpts.Title = "Skipped"
		statusOpts.Summary = "is skipping this commit."
	case "neutral":
		statusOpts.Title = "Unknown"
		statusOpts.Summary = "doesn't know what happened with this commit."
	}

	if statusOpts.Status == "in_progress" {
		statusOpts.Title = "CI has Started"
		statusOpts.Summary = "is running."
	}

	onPr := ""
	if statusOpts.OriginalPipelineRunName != "" {
		onPr = "/" + statusOpts.OriginalPipelineRunName
	}
	statusOpts.Summary = fmt.Sprintf("%s%s %s", pacOpts.ApplicationName, onPr, statusOpts.Summary)

	return v.createStatusCommit(event, pacOpts, statusOpts)
}

func (v *Provider) createStatusCommit(event *info.Event, pacopts *info.PacOpts, status provider.StatusOpts) error {
	state := gitea.StatusState(status.Conclusion)
	switch status.Conclusion {
	case "skipped", "neutral":
		state = gitea.StatusSuccess // We don't have a choice than setting as success, no pending here.c
	}
	if status.Status == "in_progress" {
		state = gitea.StatusPending
	}

	gStatus := gitea.CreateStatusOption{
		State:       state,
		TargetURL:   status.DetailsURL,
		Description: status.Title,
		Context:     getCheckName(status, pacopts),
	}
	if _, _, err := v.Client.CreateStatus(event.Organization, event.Repository, event.SHA, gStatus); err != nil {
		return err
	}

	if status.Text != "" && event.EventType == "pull_request" {
		_, _, err := v.Client.CreateIssueComment(event.Organization, event.Repository,
			int64(event.PullRequestNumber), gitea.CreateIssueCommentOption{
				Body: fmt.Sprintf("%s<br>%s", status.Summary, status.Text),
			},
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// TODO: move to common since used in github and here
func getCheckName(status provider.StatusOpts, pacopts *info.PacOpts) string {
	if pacopts.ApplicationName != "" {
		if status.OriginalPipelineRunName == "" {
			return pacopts.ApplicationName
		}
		return fmt.Sprintf("%s / %s", pacopts.ApplicationName, status.OriginalPipelineRunName)
	}
	return status.OriginalPipelineRunName
}

func (v *Provider) GetTektonDir(_ context.Context, event *info.Event, path string) (string, error) {
	tektonDirSha := ""
	rootobjects, _, err := v.Client.GetTrees(event.Organization, event.Repository, event.SHA, false)
	if err != nil {
		return "", err
	}
	for _, object := range rootobjects.Entries {
		if object.Path == path {
			if object.Type != "tree" {
				return "", fmt.Errorf("%s has been found but is not a directory", path)
			}
			tektonDirSha = object.SHA
		}
	}

	// If we didn't find a .tekton directory then just silently ignore the error.
	if tektonDirSha == "" {
		return "", nil
	}
	// Get all files in the .tekton directory recursively
	// TODO: figure out if there is a object limit we need to handle here
	tektonDirObjects, _, err := v.Client.GetTrees(event.Organization, event.Repository, tektonDirSha, true)
	if err != nil {
		return "", err
	}
	return v.concatAllYamlFiles(tektonDirObjects.Entries, event)
}

func (v *Provider) concatAllYamlFiles(objects []gitea.GitEntry, event *info.Event) (string,
	error,
) {
	var allTemplates string

	for _, value := range objects {
		if strings.HasSuffix(value.Path, ".yaml") ||
			strings.HasSuffix(value.Path, ".yml") {
			data, err := v.getObject(value.SHA, event)
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

func (v *Provider) getObject(sha string, event *info.Event) ([]byte, error) {
	blob, _, err := v.Client.GetBlob(event.Organization, event.Repository, sha)
	if err != nil {
		return nil, err
	}
	decoded, err := base64.StdEncoding.DecodeString(blob.Content)
	if err != nil {
		return nil, err
	}
	return decoded, err
}

func (v *Provider) GetFileInsideRepo(_ context.Context, runevent *info.Event, path, target string) (string, error) {
	ref := runevent.SHA
	if target != "" {
		ref = runevent.BaseBranch
	}

	content, _, err := v.Client.GetContents(runevent.Organization, runevent.Repository, ref, path)
	if err != nil {
		return "", err
	}
	// base64 decode to string
	decoded, err := base64.StdEncoding.DecodeString(*content.Content)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

func (v *Provider) GetCommitInfo(ctx context.Context, runevent *info.Event) error {
	if v.Client == nil {
		return fmt.Errorf("no gitea client has been initiliazed, " +
			"exiting... (hint: did you forget setting a secret on your repo?)")
	}

	sha := runevent.SHA
	if sha == "" && runevent.HeadBranch != "" {
		branchinfo, _, err := v.Client.GetRepoBranch(runevent.Organization, runevent.Repository, runevent.HeadBranch)
		if err != nil {
			return err
		}
		sha = branchinfo.Commit.ID
	} else if sha == "" && runevent.PullRequestNumber != 0 {
		pr, _, err := v.Client.GetPullRequest(runevent.Organization, runevent.Repository, int64(runevent.PullRequestNumber))
		if err != nil {
			return err
		}
		runevent.SHA = pr.Head.Sha
		runevent.HeadBranch = pr.Head.Ref
		runevent.BaseBranch = pr.Base.Ref
		sha = pr.Head.Sha
	}
	commit, _, err := v.Client.GetSingleCommit(runevent.Organization, runevent.Repository, sha)
	if err != nil {
		return err
	}
	runevent.SHAURL = commit.HTMLURL
	runevent.SHATitle = strings.Split(commit.RepoCommit.Message, "\n\n")[0]
	runevent.SHA = commit.SHA
	return nil
}

func (v *Provider) GetFiles(ctx context.Context, runevent *info.Event) ([]string, error) {
	// TODO: figure out a way
	return []string{}, fmt.Errorf("GetFiles is not supported on Gitea")
}
