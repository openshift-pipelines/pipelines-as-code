package bitbucketcloud

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/ktrysmt/go-bitbucket"
	"github.com/mitchellh/mapstructure"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/changedfiles"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/events"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/bitbucketcloud/types"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
)

var _ provider.Interface = (*Provider)(nil)

type Provider struct {
	Client        *bitbucket.Client
	Logger        *zap.SugaredLogger
	run           *params.Run
	Token, APIURL *string
	Username      *string
	provenance    string
}

// CheckPolicyAllowing TODO: Implement ME.
func (v *Provider) CheckPolicyAllowing(_ context.Context, _ *info.Event, _ []string) (bool, string) {
	return false, ""
}

// GetTaskURI TODO: Implement ME.
func (v *Provider) GetTaskURI(_ context.Context, _ *info.Event, _ string) (bool, string, error) {
	return false, "", nil
}

const taskStatusTemplate = `| **Status** | **Duration** | **Name** |
| --- | --- | --- |
{{range $taskrun := .TaskRunList }}|{{ formatCondition $taskrun.PipelineRunTaskRunStatus.Status.Conditions }}|{{ formatDuration $taskrun.PipelineRunTaskRunStatus.Status.StartTime $taskrun.PipelineRunTaskRunStatus.Status.CompletionTime }}|{{ $taskrun.ConsoleLogURL }}|
{{ end }}`

func (v *Provider) Validate(_ context.Context, _ *params.Run, _ *info.Event) error {
	return nil
}

func (v *Provider) SetLogger(logger *zap.SugaredLogger) {
	v.Logger = logger
}

func (v *Provider) GetConfig() *info.ProviderConfig {
	return &info.ProviderConfig{
		TaskStatusTMPL: taskStatusTemplate,
		APIURL:         bitbucket.DEFAULT_BITBUCKET_API_BASE_URL,
		Name:           "bitbucket-cloud",
	}
}

func (v *Provider) CreateStatus(_ context.Context, event *info.Event, statusopts provider.StatusOpts) error {
	switch statusopts.Conclusion {
	case "skipped":
		statusopts.Conclusion = "STOPPED"
		statusopts.Title = "➖ Skipping this commit"
	case "neutral":
		statusopts.Conclusion = "STOPPED"
		statusopts.Title = "➖ CI has stopped"
	case "failure":
		statusopts.Conclusion = "FAILED"
		statusopts.Title = "❌ Failed"
	case "pending":
		statusopts.Conclusion = "INPROGRESS"
		statusopts.Title = "⚡ CI has started"
	case "success":
		statusopts.Conclusion = "SUCCESSFUL"
		statusopts.Title = "✅ Commit has been validated"
	case "completed":
		statusopts.Conclusion = "SUCCESSFUL"
		statusopts.Title = "✅ Completed"
	}
	detailsURL := event.Provider.URL
	if statusopts.DetailsURL != "" {
		detailsURL = statusopts.DetailsURL
	}

	cso := &bitbucket.CommitStatusOptions{
		Key:         v.run.Info.Pac.ApplicationName,
		Url:         detailsURL,
		State:       statusopts.Conclusion,
		Description: statusopts.Title,
	}
	cmo := &bitbucket.CommitsOptions{
		Owner:    event.Organization,
		RepoSlug: event.Repository,
		Revision: event.SHA,
	}

	if v.Client == nil {
		return fmt.Errorf("no token has been set, cannot set status")
	}

	_, err := v.Client.Repositories.Commits.CreateCommitStatus(cmo, cso)
	if err != nil {
		return err
	}
	if statusopts.Conclusion != "STOPPED" && statusopts.Status == "completed" &&
		statusopts.Text != "" && event.EventType == triggertype.PullRequest.String() {
		onPr := ""
		if statusopts.OriginalPipelineRunName != "" {
			onPr = "/" + statusopts.OriginalPipelineRunName
		}
		_, err = v.Client.Repositories.PullRequests.AddComment(
			&bitbucket.PullRequestCommentOptions{
				Owner:         event.Organization,
				RepoSlug:      event.Repository,
				PullRequestID: strconv.Itoa(event.PullRequestNumber),
				Content:       fmt.Sprintf("**%s%s** - %s\n\n%s", v.run.Info.Pac.ApplicationName, onPr, statusopts.Title, statusopts.Text),
			})
		if err != nil {
			return err
		}
	}
	return nil
}

func (v *Provider) GetTektonDir(_ context.Context, event *info.Event, path, provenance string) (string, error) {
	v.provenance = provenance
	repositoryFiles, err := v.getDir(event, path)
	if err != nil {
		return "", err
	}

	return v.concatAllYamlFiles(repositoryFiles, event)
}

func (v *Provider) getDir(event *info.Event, path string) ([]bitbucket.RepositoryFile, error) {
	// default set provenance from the SHA
	revision := event.SHA
	if v.provenance == "default_branch" {
		revision = event.DefaultBranch
		v.Logger.Infof("Using PipelineRun definition from default_branch: %s", event.DefaultBranch)
	} else {
		v.Logger.Infof("Using PipelineRun definition from source pull request SHA: %s", event.SHA)
	}
	repoFileOpts := &bitbucket.RepositoryFilesOptions{
		Owner:    event.Organization,
		RepoSlug: event.Repository,
		Ref:      revision,
		Path:     path,
	}

	repositoryFiles, err := v.Client.Repositories.Repository.ListFiles(repoFileOpts)
	if err != nil {
		return nil, err
	}
	return repositoryFiles, nil
}

func (v *Provider) GetFileInsideRepo(_ context.Context, event *info.Event, path, _ string) (string, error) {
	revision := event.SHA
	if v.provenance == "default_branch" {
		revision = event.DefaultBranch
	}
	return v.getBlob(event, revision, path)
}

func (v *Provider) SetClient(_ context.Context, run *params.Run, event *info.Event, _ *v1alpha1.Repository, _ *events.EventEmitter) error {
	if event.Provider.Token == "" {
		return fmt.Errorf("no git_provider.secret has been set in the repo crd")
	}
	if event.Provider.User == "" {
		return fmt.Errorf("no git_provider.user has been in repo crd")
	}
	v.Client = bitbucket.NewBasicAuth(event.Provider.User, event.Provider.Token)
	v.Token = &event.Provider.Token
	v.Username = &event.Provider.User
	v.run = run
	return nil
}

func (v *Provider) GetCommitInfo(_ context.Context, event *info.Event) error {
	branchortag := event.SHA
	if branchortag == "" {
		branchortag = event.HeadBranch
	}
	response, err := v.Client.Repositories.Commits.GetCommits(&bitbucket.CommitsOptions{
		Owner:       event.Organization,
		RepoSlug:    event.Repository,
		Branchortag: branchortag,
	})
	if err != nil {
		return err
	}
	commitMap, ok := response.(map[string]interface{})
	if !ok {
		return fmt.Errorf("cannot convert")
	}
	values, ok := commitMap["values"].([]interface{})
	if !ok {
		return fmt.Errorf("cannot convert")
	}
	if len(values) == 0 {
		return fmt.Errorf("we did not get commit information from commit: %s", event.SHA)
	}
	commitinfo := &types.Commit{}
	err = mapstructure.Decode(values[0], commitinfo)
	if err != nil {
		return err
	}

	// Some silliness since we get first the account id and we fill it properly after
	event.SHATitle = commitinfo.Message
	event.SHAURL = commitinfo.Links.HTML.HRef
	event.SHA = commitinfo.Hash

	// now to get the default branch from repository.Get
	repo, err := v.Client.Repositories.Repository.Get(&bitbucket.RepositoryOptions{
		Owner:    event.Organization,
		RepoSlug: event.Repository,
	})
	if err != nil {
		return err
	}
	event.DefaultBranch = repo.Mainbranch.Name
	return nil
}

func (v *Provider) concatAllYamlFiles(objects []bitbucket.RepositoryFile, event *info.Event) (string, error) {
	var allTemplates string

	revision := event.SHA
	if v.provenance == "default_branch" {
		revision = event.DefaultBranch
	}
	for _, value := range objects {
		if value.Type == "commit_directory" {
			objects, err := v.getDir(event, value.Path)
			if err != nil {
				return "", err
			}
			subdirdata, err := v.concatAllYamlFiles(objects, event)
			if err != nil {
				return "", err
			}
			if allTemplates != "" && !strings.HasPrefix(subdirdata, "---") {
				allTemplates += "---"
			}
			allTemplates += fmt.Sprintf("\n%s\n", subdirdata)
		} else if strings.HasSuffix(value.Path, ".yaml") ||
			strings.HasSuffix(value.Path, ".yml") {
			data, err := v.getBlob(event, revision, value.Path)
			if err != nil {
				return "", err
			}
			var i any
			if err := yaml.Unmarshal([]byte(data), &i); err != nil {
				return "", fmt.Errorf("error unmarshalling yaml file %s: %w", value.Path, err)
			}

			if allTemplates != "" && !strings.HasPrefix(data, "---") {
				allTemplates += "---"
			}
			allTemplates += "\n" + data + "\n"
		}
	}
	return allTemplates, nil
}

func (v *Provider) getBlob(runevent *info.Event, ref, path string) (string, error) {
	blob, err := v.Client.Repositories.Repository.GetFileBlob(&bitbucket.RepositoryBlobOptions{
		Owner:    runevent.Organization,
		RepoSlug: runevent.Repository,
		Ref:      ref,
		Path:     path,
	})
	if err != nil {
		return "", fmt.Errorf("cannot find %s on branch %s in repo %s/%s", path, ref, runevent.Organization, runevent.Repository)
	}
	return blob.String(), nil
}

func (v *Provider) GetFiles(_ context.Context, _ *info.Event) (changedfiles.ChangedFiles, error) {
	return changedfiles.ChangedFiles{}, nil
}

func (v *Provider) CreateToken(_ context.Context, _ []string, _ *info.Event) (string, error) {
	return "", nil
}
