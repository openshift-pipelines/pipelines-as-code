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

	"code.gitea.io/sdk/gitea"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/changedfiles"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/events"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/opscomments"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
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
	Client           *gitea.Client
	Logger           *zap.SugaredLogger
	Token            *string
	giteaInstanceURL string
	// only exposed for e2e tests
	Password     string
	repo         *v1alpha1.Repository
	eventEmitter *events.EventEmitter
	run          *params.Run
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
	v.eventEmitter = emitter
	v.repo = repo
	v.run = run
	return nil
}

func (v *Provider) CreateStatus(_ context.Context, event *info.Event, statusOpts provider.StatusOpts) error {
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
	statusOpts.Summary = fmt.Sprintf("%s%s %s", v.run.Info.Pac.ApplicationName, onPr, statusOpts.Summary)

	return v.createStatusCommit(event, v.run.Info.Pac, statusOpts)
}

func (v *Provider) createStatusCommit(event *info.Event, pacopts *info.PacOpts, status provider.StatusOpts) error {
	state := gitea.StatusState(status.Conclusion)
	switch status.Conclusion {
	case "neutral":
		state = gitea.StatusSuccess // We don't have a choice than setting as success, no pending here.c
	case "pending":
		if status.Title != "" {
			state = gitea.StatusPending
		}
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
	if event.EventType == triggertype.OkToTest.String() || event.EventType == triggertype.Retest.String() ||
		event.EventType == triggertype.Cancel.String() {
		event.EventType = triggertype.PullRequest.String()
	}
	if opscomments.IsAnyOpsEventType(event.EventType) {
		event.EventType = triggertype.PullRequest.String()
	}

	if status.Text != "" && (event.EventType == triggertype.PullRequest.String() || event.TriggerTarget == triggertype.PullRequest) {
		status.Text = strings.ReplaceAll(strings.TrimSpace(status.Text), "<br>", "\n")
		_, _, err := v.Client.CreateIssueComment(event.Organization, event.Repository,
			int64(event.PullRequestNumber), gitea.CreateIssueCommentOption{
				Body: fmt.Sprintf("%s\n%s", status.Summary, status.Text),
			},
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// TODO: move to common since used in github and here.
func getCheckName(status provider.StatusOpts, pacopts *info.PacOpts) string {
	if pacopts.ApplicationName != "" {
		if status.OriginalPipelineRunName == "" {
			return pacopts.ApplicationName
		}
		return fmt.Sprintf("%s / %s", pacopts.ApplicationName, status.OriginalPipelineRunName)
	}
	return status.OriginalPipelineRunName
}

func (v *Provider) GetTektonDir(_ context.Context, event *info.Event, path, provenance string) (string, error) {
	// default set provenance from the SHA
	revision := event.SHA
	if provenance == "default_branch" {
		revision = event.DefaultBranch
		v.Logger.Infof("Using PipelineRun definition from default_branch: %s", event.DefaultBranch)
	} else {
		v.Logger.Infof("Using PipelineRun definition from source pull request SHA: %s", event.SHA)
	}

	tektonDirSha := ""
	rootobjects, _, err := v.Client.GetTrees(event.Organization, event.Repository, revision, false)
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
			// validate yaml
			var i any
			if err := yaml.Unmarshal(data, &i); err != nil {
				return "", fmt.Errorf("error unmarshalling yaml file %s: %w", value.Path, err)
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

func (v *Provider) GetCommitInfo(_ context.Context, runevent *info.Event) error {
	if v.Client == nil {
		return fmt.Errorf("no gitea client has been initialized, " +
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

func ShouldGetNextPage(resp *gitea.Response, currentPage int) (bool, int) {
	val, exists := resp.Response.Header[http.CanonicalHeaderKey("x-pagecount")]
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
	Commits []gitea.PayloadCommit `json:"commits,omitempty"`
}

func (v *Provider) GetFiles(_ context.Context, runevent *info.Event) (changedfiles.ChangedFiles, error) {
	changedFiles := changedfiles.ChangedFiles{}

	//nolint:exhaustive // we don't need to handle all cases
	switch runevent.TriggerTarget {
	case triggertype.PullRequest:
		opt := gitea.ListPullRequestFilesOptions{ListOptions: gitea.ListOptions{Page: 1, PageSize: 50}}
		shouldGetNextPage := false
		for {
			prChangedFiles, resp, err := v.Client.ListPullRequestFiles(runevent.Organization, runevent.Repository, int64(runevent.PullRequestNumber), opt)
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
