package gitea

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/changedfiles"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/events"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/opscomments"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	providerMetrics "github.com/openshift-pipelines/pipelines-as-code/pkg/provider/metrics"
	"go.uber.org/zap"
)

const (
	taskStatusTemplate = `
<table>
  <tr><th>Status</th><th>Duration</th><th>Name</th></tr>

{{- range $taskrun := .TaskRunList }}
<tr>
<td>{{ formatCondition $taskrun.PipelineRunTaskRunStatus.Status.Conditions }}</td>
<td>{{ formatDuration $taskrun.PipelineRunTaskRunStatus.Status.StartTime $taskrun.Status.CompletionTime }}</td><td>

{{ $taskrun.ConsoleLogURL }}

</td></tr>
{{- end }}
</table>`
)

// validate the struct to interface.
var _ provider.Interface = (*Provider)(nil)

type Provider struct {
	giteaClient      *forgejo.Client
	Logger           *zap.SugaredLogger
	pacInfo          *info.PacOpts
	Token            *string
	giteaInstanceURL string
	// only exposed for e2e tests
	Password     string
	repo         *v1alpha1.Repository
	eventEmitter *events.EventEmitter
	run          *params.Run
	triggerEvent string
}

func (v *Provider) Client() *forgejo.Client {
	providerMetrics.RecordAPIUsage(
		v.Logger,
		// URL used instead of "gitea" to differentiate in the case of a CI cluster which
		// serves multiple Gitea instances
		v.giteaInstanceURL,
		v.triggerEvent,
		v.repo,
	)
	return v.giteaClient
}

func (v *Provider) SetGiteaClient(client *forgejo.Client) {
	v.giteaClient = client
}

func (v *Provider) CreateComment(_ context.Context, event *info.Event, commit, updateMarker string) error {
	if v.giteaClient == nil {
		return fmt.Errorf("no gitea client has been initialized")
	}

	if event.PullRequestNumber == 0 {
		return fmt.Errorf("create comment only works on pull requests")
	}

	// List comments of the PR
	if updateMarker != "" {
		comments, _, err := v.Client().ListIssueComments(event.Organization, event.Repository, int64(event.PullRequestNumber), forgejo.ListIssueCommentOptions{})
		if err != nil {
			return err
		}

		re := regexp.MustCompile(updateMarker)
		for _, comment := range comments {
			if re.MatchString(comment.Body) {
				_, _, err := v.Client().EditIssueComment(event.Organization, event.Repository, comment.ID, forgejo.EditIssueCommentOption{
					Body: commit,
				})
				return err
			}
		}
	}

	_, _, err := v.Client().CreateIssueComment(event.Organization, event.Repository, int64(event.PullRequestNumber), forgejo.CreateIssueCommentOption{
		Body: commit,
	})

	return err
}

func (v *Provider) SetPacInfo(pacInfo *info.PacOpts) {
	v.pacInfo = pacInfo
}

// GetTaskURI TODO: Implement ME.
func (v *Provider) GetTaskURI(_ context.Context, _ *info.Event, _ string) (bool, string, error) {
	return false, "", nil
}

func (v *Provider) SetLogger(logger *zap.SugaredLogger) {
	v.Logger = logger
}

func (v *Provider) Validate(_ context.Context, _ *params.Run, _ *info.Event) error {
	// TODO: figure out why gitea doesn't work with mac validation as github which seems to be the same
	v.Logger.Debug("no secret and signature found, skipping validation for gitea")
	return nil
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

func (v *Provider) SetClient(_ context.Context, run *params.Run, runevent *info.Event, repo *v1alpha1.Repository, emitter *events.EventEmitter) error {
	var err error
	apiURL := runevent.Provider.URL
	// password is not exposed to CRD, it's only used from the e2e tests
	if v.Password != "" && runevent.Provider.User != "" {
		v.giteaClient, err = forgejo.NewClient(apiURL, forgejo.SetBasicAuth(runevent.Provider.User, v.Password))
	} else {
		if runevent.Provider.Token == "" {
			return fmt.Errorf("no git_provider.secret has been set in the repo crd")
		}
		v.giteaClient, err = forgejo.NewClient(apiURL, forgejo.SetToken(runevent.Provider.Token))
	}
	if err != nil {
		return err
	}

	// Added log for security audit purposes to log client access when a token is used
	run.Clients.Log.Infof("gitea: initialized API client with provided credentials user=%s providerURL=%s", runevent.Provider.User, apiURL)

	v.giteaInstanceURL = runevent.Provider.URL
	v.eventEmitter = emitter
	v.repo = repo
	v.run = run
	v.triggerEvent = runevent.EventType
	return nil
}

func (v *Provider) CreateStatus(_ context.Context, event *info.Event, statusOpts provider.StatusOpts) error {
	if v.giteaClient == nil {
		return fmt.Errorf("cannot set status on gitea no token or url set")
	}
	switch statusOpts.Conclusion {
	case "success":
		statusOpts.Title = "Success"
		statusOpts.Summary = "has <b>successfully</b> validated your commit."
	case "failure":
		statusOpts.Title = "Failed"
		statusOpts.Summary = "has <b>failed</b>."
	case "pending":
		// for concurrency set title as pending
		if statusOpts.Title == "" {
			statusOpts.Title = "Pending"
		}
		// for unauthorized user set title as Pending approval
		statusOpts.Summary = "is skipping this commit."
	case "neutral":
		statusOpts.Title = "Unknown"
		statusOpts.Summary = "doesn't know what happened with this commit."
	}

	if statusOpts.Status == "in_progress" {
		statusOpts.Title = "CI has Started"
		statusOpts.Summary = "is running.\n"
	}

	onPr := ""
	if statusOpts.PipelineRunName != "" {
		onPr = fmt.Sprintf("/%s", statusOpts.PipelineRunName)
	}
	// gitea show weirdly the <br>
	statusOpts.Summary = fmt.Sprintf("%s%s %s", v.pacInfo.ApplicationName, onPr, statusOpts.Summary)

	return v.createStatusCommit(event, v.pacInfo, statusOpts)
}

func (v *Provider) createStatusCommit(event *info.Event, pacopts *info.PacOpts, status provider.StatusOpts) error {
	state := forgejo.StatusState(status.Conclusion)
	switch status.Conclusion {
	case "neutral":
		state = forgejo.StatusSuccess // We don't have a choice than setting as success, no pending here.c
	case "pending":
		if status.Title != "" {
			state = forgejo.StatusPending
		}
	}
	if status.Status == "in_progress" {
		state = forgejo.StatusPending
	}

	gStatus := forgejo.CreateStatusOption{
		State:       state,
		TargetURL:   status.DetailsURL,
		Description: status.Title,
		Context:     provider.GetCheckName(status, pacopts),
	}

	// Retry logic for transient errors (e.g., "user does not exist" under concurrent load)
	maxRetries := 3
	var lastErr error
	for i := range maxRetries {
		if _, _, err := v.Client().CreateStatus(event.Organization, event.Repository, event.SHA, gStatus); err != nil {
			lastErr = err
			// Only retry on transient "user does not exist" errors
			if strings.Contains(err.Error(), "user does not exist") {
				v.Logger.Warnf("CreateStatus failed with transient error, retrying %d/%d: %v", i+1, maxRetries, err)
				time.Sleep(time.Duration(i+1) * 500 * time.Millisecond)
				continue
			}
			return err
		}
		lastErr = nil
		break
	}
	if lastErr != nil {
		return lastErr
	}

	eventType := triggertype.IsPullRequestType(event.EventType)
	if opscomments.IsAnyOpsEventType(eventType.String()) {
		eventType = triggertype.PullRequest
	}
	if status.Text != "" && (eventType == triggertype.PullRequest || event.TriggerTarget == triggertype.PullRequest) {
		status.Text = strings.ReplaceAll(strings.TrimSpace(status.Text), "<br>", "\n")
		_, _, err := v.Client().CreateIssueComment(event.Organization, event.Repository,
			int64(event.PullRequestNumber), forgejo.CreateIssueCommentOption{
				Body: fmt.Sprintf("%s\n%s", status.Summary, status.Text),
			},
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (v *Provider) GetTektonDir(_ context.Context, event *info.Event, path, provenance string) (string, error) {
	// default set provenance from the SHA
	revision := event.SHA
	if provenance == "default_branch" {
		revision = event.DefaultBranch
		v.Logger.Infof("Using PipelineRun definition from default_branch: %s", event.DefaultBranch)
	} else {
		v.Logger.Infof("Using PipelineRun definition from source %s commit SHA: %s", event.TriggerTarget.String(), event.SHA)
	}

	tektonDirSha := ""
	opt := forgejo.GetTreesOptions{
		Recursive: false,
	}
	rootobjects, _, err := v.Client().GetTrees(event.Organization, event.Repository, revision, opt)
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
	opts := forgejo.GetTreesOptions{Recursive: false}
	tektonDirObjects, _, err := v.Client().GetTrees(event.Organization, event.Repository, tektonDirSha, opts)
	if err != nil {
		return "", err
	}
	return v.concatAllYamlFiles(tektonDirObjects.Entries, event)
}

func (v *Provider) concatAllYamlFiles(objects []forgejo.GitEntry, event *info.Event) (string,
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
			if err := provider.ValidateYaml(data, value.Path); err != nil {
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
	blob, _, err := v.Client().GetBlob(event.Organization, event.Repository, sha)
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

	content, _, err := v.Client().GetContents(runevent.Organization, runevent.Repository, ref, path)
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

func (v *Provider) GetCommitInfo(_ context.Context, runevent *info.Event) error {
	if v.giteaClient == nil {
		return fmt.Errorf("no gitea client has been initialized, " +
			"exiting... (hint: did you forget setting a secret on your repo?)")
	}

	sha := runevent.SHA
	if sha == "" && runevent.HeadBranch != "" {
		branchinfo, _, err := v.Client().GetRepoBranch(runevent.Organization, runevent.Repository, runevent.HeadBranch)
		if err != nil {
			return err
		}
		sha = branchinfo.Commit.ID
	} else if sha == "" && runevent.PullRequestNumber != 0 {
		pr, _, err := v.Client().GetPullRequest(runevent.Organization, runevent.Repository, int64(runevent.PullRequestNumber))
		if err != nil {
			return err
		}
		runevent.SHA = pr.Head.Sha
		runevent.HeadBranch = pr.Head.Ref
		runevent.BaseBranch = pr.Base.Ref
		sha = pr.Head.Sha
	}
	commit, _, err := v.Client().GetSingleCommit(runevent.Organization, runevent.Repository, sha)
	if err != nil {
		return err
	}
	runevent.SHAURL = commit.HTMLURL
	runevent.SHATitle = strings.Split(commit.RepoCommit.Message, "\n\n")[0]
	runevent.SHA = commit.SHA

	// Populate full commit information for LLM context
	runevent.SHAMessage = commit.RepoCommit.Message
	if commit.RepoCommit.Author != nil {
		runevent.SHAAuthorName = commit.RepoCommit.Author.Name
		runevent.SHAAuthorEmail = commit.RepoCommit.Author.Email
		if commit.RepoCommit.Author.Date != "" {
			if authorDate, err := time.Parse(time.RFC3339, commit.RepoCommit.Author.Date); err == nil {
				runevent.SHAAuthorDate = authorDate
			}
		}
	}
	if commit.RepoCommit.Committer != nil {
		runevent.SHACommitterName = commit.RepoCommit.Committer.Name
		runevent.SHACommitterEmail = commit.RepoCommit.Committer.Email
		if commit.RepoCommit.Committer.Date != "" {
			if committerDate, err := time.Parse(time.RFC3339, commit.RepoCommit.Committer.Date); err == nil {
				runevent.SHACommitterDate = committerDate
			}
		}
	}
	runevent.HasSkipCommand = provider.SkipCI(commit.RepoCommit.Message)

	return nil
}

func ShouldGetNextPage(resp *forgejo.Response, currentPage int) (bool, int) {
	val, exists := resp.Header[http.CanonicalHeaderKey("x-pagecount")]
	if !exists {
		return false, 0
	}
	i, err := strconv.Atoi(val[0])
	if err != nil {
		return false, 0
	}
	if i >= currentPage {
		return false, i
	}
	return true, (currentPage + 1)
}

type PushPayload struct {
	Commits []forgejo.PayloadCommit `json:"commits,omitempty"`
}

func (v *Provider) GetFiles(_ context.Context, runevent *info.Event) (changedfiles.ChangedFiles, error) {
	changedFiles := changedfiles.ChangedFiles{}

	//nolint:exhaustive // we don't need to handle all cases
	switch runevent.TriggerTarget {
	case triggertype.PullRequest, triggertype.PullRequestClosed:
		opt := forgejo.ListPullRequestFilesOptions{ListOptions: forgejo.ListOptions{Page: 1, PageSize: 50}}
		shouldGetNextPage := false
		for {
			prChangedFiles, resp, err := v.Client().ListPullRequestFiles(runevent.Organization, runevent.Repository, int64(runevent.PullRequestNumber), opt)
			if err != nil {
				return changedfiles.ChangedFiles{}, err
			}
			for j := range prChangedFiles {
				changedFiles.All = append(changedFiles.All, prChangedFiles[j].Filename)
				if prChangedFiles[j].Status == "added" {
					changedFiles.Added = append(changedFiles.Added, prChangedFiles[j].Filename)
				}
				if prChangedFiles[j].Status == "deleted" {
					changedFiles.Deleted = append(changedFiles.Deleted, prChangedFiles[j].Filename)
				}
				if prChangedFiles[j].Status == "changed" {
					changedFiles.Modified = append(changedFiles.Modified, prChangedFiles[j].Filename)
				}
				if prChangedFiles[j].Status == "renamed" {
					changedFiles.Renamed = append(changedFiles.Renamed, prChangedFiles[j].Filename)
				}
			}

			shouldGetNextPage, opt.Page = ShouldGetNextPage(resp, opt.Page)
			if !shouldGetNextPage {
				break
			}
		}
	case triggertype.Push:
		pushPayload := PushPayload{}
		err := json.Unmarshal(runevent.Request.Payload, &pushPayload)
		if err != nil {
			v.Logger.Errorf("failed to unmarshal the push payload to get changed files - %v", err)
			return changedfiles.ChangedFiles{}, fmt.Errorf("failed to unmarshal the push payload to get changed files - %w", err)
		}

		for _, commit := range pushPayload.Commits {
			for _, file := range commit.Added {
				changedFiles.All = append(changedFiles.All, file)
				changedFiles.Added = append(changedFiles.Added, file)
			}
			for _, file := range commit.Modified {
				changedFiles.All = append(changedFiles.All, file)
				changedFiles.Modified = append(changedFiles.Modified, file)
			}
			for _, file := range commit.Removed {
				changedFiles.All = append(changedFiles.All, file)
				changedFiles.Deleted = append(changedFiles.Deleted, file)
			}
		}
	default:
		v.Logger.Errorf("unable to get changed files. Unknown trigger type of '%s'. Expected pull_request or push", runevent.TriggerTarget)
		return changedFiles, fmt.Errorf("unable to get changed files. Unknown trigger type of '%s'. Expected pull_request or push", runevent.TriggerTarget)
	}

	return changedFiles, nil
}

func (v *Provider) CreateToken(_ context.Context, _ []string, _ *info.Event) (string, error) {
	return "", nil
}

func (v *Provider) GetTemplate(commentType provider.CommentType) string {
	return provider.GetHTMLTemplate(commentType)
}
