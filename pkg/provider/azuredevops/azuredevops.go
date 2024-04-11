package azuredevops

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/changedfiles"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/events"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	types "github.com/openshift-pipelines/pipelines-as-code/pkg/provider/azuredevops/types"
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

var _ provider.Interface = (*Provider)(nil)

type Provider struct {
	Client git.Client
	ctx    context.Context
	Logger *zap.SugaredLogger
	Token  *string
	run    *params.Run
}

// CreateStatus implements provider.Interface.
func (v *Provider) CreateStatus(ctx context.Context, event *info.Event, statusOpts provider.StatusOpts) error {
	if v.Client == nil {
		return fmt.Errorf("cannot set status on azuredevops no token or url set")
	}

	var gitStatusState git.GitStatusState

	switch statusOpts.Conclusion {
	case "success":
		statusOpts.Title = "Success"
		statusOpts.Summary = "has successfully validated your commit."
		gitStatusState = git.GitStatusStateValues.Succeeded
	case "failure":
		statusOpts.Title = "failure"
		statusOpts.Summary = "has failed."
		gitStatusState = git.GitStatusStateValues.Failed
	case "pending":
		statusOpts.Title = "Pending"
		statusOpts.Summary = "is waiting to start."
		gitStatusState = git.GitStatusStateValues.Pending
	case "error":
		statusOpts.Title = "Error"
		statusOpts.Summary = "encountered an error during processing."
		gitStatusState = git.GitStatusStateValues.Error
	case "notApplicable":
		statusOpts.Title = "Not Applicable"
		statusOpts.Summary = "is not applicable for this commit."
		gitStatusState = git.GitStatusStateValues.NotApplicable
	default:
		statusOpts.Title = "Unknown"
		statusOpts.Summary = "doesn't know what happened with this commit."
		gitStatusState = git.GitStatusStateValues.NotSet
	}

	if statusOpts.Status == "in_progress" {
		gitStatusState = git.GitStatusStateValues.Pending
		statusOpts.Title = "In Progress"
		statusOpts.Summary = "is in progress."
	}

	onPr := ""
	if statusOpts.PipelineRunName != "" {
		onPr = fmt.Sprintf("/%s", statusOpts.PipelineRunName)
	}
	statusOpts.Summary = fmt.Sprintf("%s%s %s", v.run.Info.Pac.ApplicationName, onPr, statusOpts.Summary)
	genreValue := "PAC"

	switch event.EventType {
	case "git.push":
		gitStatus := git.GitStatus{
			State:       &gitStatusState,
			TargetUrl:   &event.URL,
			Description: &statusOpts.Summary,
			Context: &git.GitStatusContext{
				Name:  &statusOpts.Title,
				Genre: &genreValue,
			},
		}
		commitStatusArgs := git.CreateCommitStatusArgs{
			Project:                 &event.ProjectId,
			RepositoryId:            &event.RepositoryId,
			CommitId:                &event.SHA,
			GitCommitStatusToCreate: &gitStatus,
		}
		if _, err := v.Client.CreateCommitStatus(ctx, commitStatusArgs); err != nil {
			return fmt.Errorf("failed to create commit status: %v", err)
		}
	case "git.pullrequest.created", "git.pullrequest.updated":
		gitPullRequestStatusArgs := git.GetPullRequestStatusesArgs{
			PullRequestId: &event.PullRequestNumber,
			Project:       &event.ProjectId,
			RepositoryId:  &event.RepositoryId,
		}

		status, err := v.Client.GetPullRequestStatuses(ctx, gitPullRequestStatusArgs)
		if err != nil {
			return fmt.Errorf("failed to fetch pull request statuses: %v", err)
		}

		if status == nil || len(*status) == 0 {
			_, err := createPRStatus(ctx, v, event, statusOpts, gitStatusState, genreValue)
			if err != nil {
				return err
			}
		} else {

			statusid := (*status)[0].Id
			path := "/" + strconv.Itoa(*statusid)

			patchDocument := []webapi.JsonPatchOperation{
				{
					Op:    &webapi.OperationValues.Remove,
					Path:  &path,
					Value: nil,
					From:  nil,
				},
			}

			gitUpdatePullRequestStatus := git.UpdatePullRequestStatusesArgs{
				PatchDocument: &patchDocument,
				Project:       &event.ProjectId,
				RepositoryId:  &event.RepositoryId,
				PullRequestId: &event.PullRequestNumber,
			}
			if err := v.Client.UpdatePullRequestStatuses(ctx, gitUpdatePullRequestStatus); err != nil {
				return fmt.Errorf("failed to update pull request status: %v", err)
			}

			_, err := createPRStatus(ctx, v, event, statusOpts, gitStatusState, genreValue)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func createPRStatus(ctx context.Context, v *Provider, event *info.Event, statusOpts provider.StatusOpts, gitStatusState git.GitStatusState, genreValue string) (bool, error) {
	gitPullRequestStatus := git.GitPullRequestStatus{
		Id:          &event.PullRequestNumber,
		TargetUrl:   &event.URL,
		Description: &statusOpts.Summary,
		State:       &gitStatusState,
		Context: &git.GitStatusContext{
			Name:  &statusOpts.Title,
			Genre: &genreValue,
		},
	}

	prStatusArgs := git.CreatePullRequestStatusArgs{
		PullRequestId: &event.PullRequestNumber,
		Project:       &event.ProjectId,
		RepositoryId:  &event.RepositoryId,
		Status:        &gitPullRequestStatus,
	}
	if _, err := v.Client.CreatePullRequestStatus(ctx, prStatusArgs); err != nil {
		return false, fmt.Errorf("failed to create pull request status: %v", err)
	}
	return true, nil
}

// CreateToken implements provider.Interface.
func (v *Provider) CreateToken(context.Context, []string, *info.Event) (string, error) {
	panic("unimplemented")
}

func (v *Provider) GetCommitInfo(ctx context.Context, event *info.Event) error {
	if v.Client == nil {
		return fmt.Errorf("no Azure DevOps client has been initialized, " +
			"exiting... (hint: did you forget setting a secret on your repo?)")
	}

	RepositoryId := event.RepositoryId
	projectId := event.ProjectId
	sha := event.SHA

	// If SHA is not provided, try to fetch it from the branch or pull request
	if sha == "" {
		if event.HeadBranch != "" {

			refName := fmt.Sprintf("refs/heads/%s", event.HeadBranch)
			refs, err := v.Client.GetRefs(ctx, git.GetRefsArgs{
				RepositoryId: &RepositoryId,
				Filter:       &refName,
				Project:      &projectId,
			})
			if err != nil {
				return fmt.Errorf("failed to get branch info: %v", err)
			}
			// Assuming refs is a pointer to a slice, we check its length like this:
			if len(refs.Value) > 0 {
				sha = *refs.Value[0].ObjectId
			}
		} else if event.PullRequestNumber != 0 {
			pr, err := v.Client.GetPullRequest(ctx, git.GetPullRequestArgs{
				RepositoryId:  &RepositoryId,
				PullRequestId: &event.PullRequestNumber,
				Project:       &projectId,
			})
			if err != nil {
				return fmt.Errorf("failed to get pull request: %v", err)
			}
			sha = *pr.LastMergeSourceCommit.CommitId
			event.HeadBranch = *pr.SourceRefName
			event.BaseBranch = *pr.TargetRefName
		}
	}
	if sha != "" {
		commit, err := v.Client.GetCommit(ctx, git.GetCommitArgs{
			CommitId:     &sha,
			RepositoryId: &RepositoryId,
			Project:      &projectId,
		})
		if err != nil {
			return fmt.Errorf("failed to get commit: %v", err)
		}
		event.SHAURL = *commit.RemoteUrl
		event.SHATitle = strings.Split(*commit.Comment, "\n\n")[0]
		event.SHA = *commit.CommitId

	} else {
		return fmt.Errorf("unable to determine commit SHA")
	}

	return nil
}

func (v *Provider) GetConfig() *info.ProviderConfig {
	return &info.ProviderConfig{
		TaskStatusTMPL: taskStatusTemplate,
		Name:           "azuredevops",
	}
}

// GetFileInsideRepo implements provider.Interface.
func (v *Provider) GetFileInsideRepo(ctx context.Context, runevent *info.Event, path, target string) (string, error) {
	panic("unimplemented")
}

func (v *Provider) GetFiles(ctx context.Context, event *info.Event) (changedfiles.ChangedFiles, error) {

	filesChanged, err := v.Client.GetChanges(ctx, git.GetChangesArgs{
		RepositoryId: &event.RepositoryId,
		CommitId:     &event.SHA,
	})

	if err != nil {
		v.Logger.Errorf("Failed to get changes for commit %s: %v", &event.SHA, err)
	}

	changesJson, err := json.Marshal(filesChanged.Changes)
	if err != nil {
		v.Logger.Errorf("Failed to marshal changes: %v", err)
		return changedfiles.ChangedFiles{}, err
	}

	var changes []types.Change
	if err := json.Unmarshal(changesJson, &changes); err != nil {
		log.Fatalf("JSON Unmarshal error: %v", err)
	}

	changedFiles := &changedfiles.ChangedFiles{}

	for _, c := range changes {

		switch c.ChangeType {
		case "edit":
			changedFiles.All = append(changedFiles.Added, c.Item.Path)
			changedFiles.Modified = append(changedFiles.Added, c.Item.Path)
		case "add":
			changedFiles.All = append(changedFiles.Added, c.Item.Path)
			changedFiles.Added = append(changedFiles.Added, c.Item.Path)
		case "delete":
			changedFiles.All = append(changedFiles.Added, c.Item.Path)
			changedFiles.Deleted = append(changedFiles.Added, c.Item.Path)
		case "rename":
			changedFiles.All = append(changedFiles.Added, c.Item.Path)
			changedFiles.Renamed = append(changedFiles.Added, c.Item.Path)
		}
	}

	return *changedFiles, nil
}

func (v *Provider) GetTaskURI(ctx context.Context, event *info.Event, uri string) (bool, string, error) {
	return false, "", nil
}

// GetTektonDir implements provider.Interface.
func (v *Provider) GetTektonDir(ctx context.Context, runevent *info.Event, path, provenance string) (string, error) {

	repositoryID := runevent.RepositoryId
	ProjectId := runevent.ProjectId
	var version string

	if provenance == "default_branch" {
		version = runevent.DefaultBranch
		v.Logger.Infof("Using Tekton definition from default branch: %s", version)
	} else {
		version = runevent.SHA
		v.Logger.Infof("Using Tekton definition from commit ID: %s", version)
	}
	// Check if the path exists and is a directory
	item, err := v.Client.GetItem(ctx, git.GetItemArgs{
		RepositoryId: &repositoryID,
		Project:      &ProjectId,
		Path:         &path,
	})

	if err != nil {
		return "", fmt.Errorf("failed to fetch the item: %v", err)
	}

	if item == nil {
		return "", fmt.Errorf("no item found under the specified path '%s' in the repository at the given branch or commit '%s'", path, version)
	}

	// Get the SHA of the directory and fetch the tree
	tree, err := v.Client.GetTree(ctx, git.GetTreeArgs{
		RepositoryId: &repositoryID,
		Project:      &ProjectId,
		Sha1:         item.ObjectId,
		Recursive:    toBoolPtr(true),
	})

	if err != nil {
		return "", fmt.Errorf("failed to fetch the tree: %v", err)
	}

	// Concatenate all YAML files found within the tree entries
	result, err := v.concatAllYamlFiles(ctx, tree.TreeEntries, repositoryID, ProjectId)
	if err != nil {
		return "", err
	}
	return result, nil

}

func toBoolPtr(b bool) *bool {
	return &b
}

func (v *Provider) concatAllYamlFiles(ctx context.Context, entries *[]git.GitTreeEntryRef, repositoryID string, projectId string) (string, error) {
	var allTemplates string

	for _, entry := range *entries {
		if *entry.GitObjectType == git.GitObjectTypeValues.Blob && (strings.HasSuffix(*entry.RelativePath, ".yaml") || strings.HasSuffix(*entry.RelativePath, ".yml")) {

			// Use the object ID (SHA) of the blob to fetch its content
			data, err := v.getObject(ctx, repositoryID, projectId, *entry.ObjectId)

			if err != nil {
				return "", err
			}
			// TODO: Validate YAML;
			// var i any
			// if err := yaml.Unmarshal(data, &i); err != nil {
			// 	return "", fmt.Errorf("error unmarshalling YAML file %s: %w", *entry.RelativePath, err)
			// }
			// Ensure each YAML document is separated by '---'
			if allTemplates != "" && !strings.HasPrefix(string(data), "---") {
				allTemplates += "\n---\n"
			}
			allTemplates += string(data) + "\n"
		}
	}
	return allTemplates, nil
}

// getObject fetches the content of a file from an Azure DevOps repository.
func (v *Provider) getObject(ctx context.Context, repositoryID string, projectId string, sha string) ([]byte, error) {
	reader, err := v.Client.GetBlobContent(ctx, git.GetBlobContentArgs{
		RepositoryId: &repositoryID,
		Project:      &projectId,
		Sha1:         &sha,
	})
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	// Read the content from the reader returned by GetBlobContent
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	return content, nil
}

// SetClient implements provider.Interface.
func (v *Provider) SetClient(_ context.Context, run *params.Run, event *info.Event, _ *v1alpha1.Repository, _ *events.EventEmitter) error {
	var err error

	if event.Provider.Token == "" {
		return fmt.Errorf("no git_provider.secret has been set in the repo crd")
	}

	organizationUrl := event.Organization
	connection := azuredevops.NewPatConnection(organizationUrl, event.Provider.Token)

	ctx := context.Background()

	v.Client, err = git.NewClient(ctx, connection)

	if err != nil {
		return err
	}

	v.Token = &event.Provider.Token
	v.run = run
	v.ctx = ctx

	return nil
}

// SetLogger implements provider.Interface.
func (v *Provider) SetLogger(logger *zap.SugaredLogger) {
	v.Logger = logger
}

// Validate implements provider.Interface.
func (v *Provider) Validate(ctx context.Context, params *params.Run, event *info.Event) error {
	return nil
}
