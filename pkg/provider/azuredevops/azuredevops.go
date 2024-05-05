package azuredevops

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
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

var _ provider.Interface = (*Provider)(nil)

type Provider struct {
	GitClient  git.Client
	CoreClient core.Client
	ctx        context.Context
	Logger     *zap.SugaredLogger
	Token      *string
	run        *params.Run
	connection *azuredevops.Connection
}

func (v *Provider) CreateStatus(ctx context.Context, event *info.Event, statusOpts provider.StatusOpts) error {
	if v.GitClient == nil {
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
			Project:                 &event.ProjectID,
			RepositoryId:            &event.RepositoryID,
			CommitId:                &event.SHA,
			GitCommitStatusToCreate: &gitStatus,
		}
		if _, err := v.GitClient.CreateCommitStatus(ctx, commitStatusArgs); err != nil {
			return fmt.Errorf("failed to create commit status: %w", err)
		}
	case "git.pullrequest.created", "git.pullrequest.updated":
		gitPullRequestStatusArgs := git.GetPullRequestStatusesArgs{
			PullRequestId: &event.PullRequestNumber,
			Project:       &event.ProjectID,
			RepositoryId:  &event.RepositoryID,
		}

		status, err := v.GitClient.GetPullRequestStatuses(ctx, gitPullRequestStatusArgs)
		if err != nil {
			return fmt.Errorf("failed to fetch pull request statuses: %w", err)
		}

		if status == nil || len(*status) == 0 {
			err := createPRStatus(ctx, v, event, statusOpts, gitStatusState, genreValue)
			if err != nil {
				return err
			}
		} else {
			// azure UpdatePullRequestStatuses only Support remove, so first remove the old status and then updated with new one

			err := updatePRStatus(ctx, status, event, v)
			if err != nil {
				return err
			}

			err = createPRStatus(ctx, v, event, statusOpts, gitStatusState, genreValue)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func updatePRStatus(ctx context.Context, status *[]git.GitPullRequestStatus, event *info.Event, v *Provider) error {
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
		Project:       &event.ProjectID,
		RepositoryId:  &event.RepositoryID,
		PullRequestId: &event.PullRequestNumber,
	}
	if err := v.GitClient.UpdatePullRequestStatuses(ctx, gitUpdatePullRequestStatus); err != nil {
		return fmt.Errorf("failed to update pull request status: %w", err)
	}
	return nil
}

func createPRStatus(ctx context.Context, v *Provider, event *info.Event, statusOpts provider.StatusOpts, gitStatusState git.GitStatusState, genreValue string) error {
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
		Project:       &event.ProjectID,
		RepositoryId:  &event.RepositoryID,
		Status:        &gitPullRequestStatus,
	}
	if _, err := v.GitClient.CreatePullRequestStatus(ctx, prStatusArgs); err != nil {
		return fmt.Errorf("failed to create pull request status: %w", err)
	}
	return nil
}

func (v *Provider) CreateToken(context.Context, []string, *info.Event) (string, error) {
	return "", nil
}

func (v *Provider) GetCommitInfo(ctx context.Context, event *info.Event) error {
	if v.GitClient == nil {
		return fmt.Errorf("no Azure DevOps client has been initialized, " +
			"exiting... (hint: did you forget setting a secret on your repo?)")
	}

	RepositoryID := event.RepositoryID
	projectID := event.ProjectID
	sha := event.SHA

	// If SHA is not provided, try to fetch it from the branch or pull request
	if sha == "" {
		if event.HeadBranch != "" {
			refName := fmt.Sprintf("refs/heads/%s", event.HeadBranch)
			refs, err := v.GitClient.GetRefs(ctx, git.GetRefsArgs{
				RepositoryId: &RepositoryID,
				Filter:       &refName,
				Project:      &projectID,
			})
			if err != nil {
				return fmt.Errorf("failed to get branch info: %w", err)
			}
			// Assuming refs is a pointer to a slice, we check its length like this:
			if len(refs.Value) > 0 {
				sha = *refs.Value[0].ObjectId
			}
		} else if event.PullRequestNumber != 0 {
			pr, err := v.GitClient.GetPullRequest(ctx, git.GetPullRequestArgs{
				RepositoryId:  &RepositoryID,
				PullRequestId: &event.PullRequestNumber,
				Project:       &projectID,
			})
			if err != nil {
				return fmt.Errorf("failed to get pull request: %w", err)
			}
			sha = *pr.LastMergeSourceCommit.CommitId
			event.HeadBranch = *pr.SourceRefName
			event.BaseBranch = *pr.TargetRefName
		}
	}
	if sha != "" {
		commit, err := v.GitClient.GetCommit(ctx, git.GetCommitArgs{
			CommitId:     &sha,
			RepositoryId: &RepositoryID,
			Project:      &projectID,
		})
		if err != nil {
			return fmt.Errorf("failed to get commit: %w", err)
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
		Name: "azuredevops",
	}
}

func (v *Provider) GetFiles(ctx context.Context, event *info.Event) (changedfiles.ChangedFiles, error) {
	filesChanged, err := v.GitClient.GetChanges(ctx, git.GetChangesArgs{
		RepositoryId: &event.RepositoryID,
		CommitId:     &event.SHA,
	})
	if err != nil {
		v.Logger.Errorf("Failed to get changes for commit %s: %w", &event.SHA, err)
	}

	changesJSON, err := json.Marshal(filesChanged.Changes)
	if err != nil {
		v.Logger.Errorf("Failed to marshal changes: %w", err)
		return changedfiles.ChangedFiles{}, err
	}

	var changes []types.Change
	if err := json.Unmarshal(changesJSON, &changes); err != nil {
		v.Logger.Errorf("JSON Unmarshal error: %w", err)
	}

	changedFiles := &changedfiles.ChangedFiles{}

	for _, c := range changes {
		switch c.ChangeType {
		case "edit":
			changedFiles.All = append(changedFiles.All, c.Item.Path)
			changedFiles.Modified = append(changedFiles.Modified, c.Item.Path)
		case "add":
			changedFiles.All = append(changedFiles.All, c.Item.Path)
			changedFiles.Added = append(changedFiles.Added, c.Item.Path)
		case "delete":
			changedFiles.All = append(changedFiles.All, c.Item.Path)
			changedFiles.Deleted = append(changedFiles.Deleted, c.Item.Path)
		case "rename":
			changedFiles.All = append(changedFiles.All, c.Item.Path)
			changedFiles.Renamed = append(changedFiles.Renamed, c.Item.Path)
		}
	}

	return *changedFiles, nil
}

// GetTaskURI TODO: Implement ME.
func (v *Provider) GetTaskURI(context.Context, *info.Event, string) (bool, string, error) {
	return false, "", nil
}

func (v *Provider) GetFileInsideRepo(ctx context.Context, runevent *info.Event, path, target string) (string, error) {
	repositoryID := runevent.RepositoryID
	ProjectID := runevent.ProjectID

	version := runevent.SHA
	versionType := git.GitVersionTypeValues.Commit
	if target != "" {
		version = runevent.BaseBranch
		versionType = git.GitVersionTypeValues.Branch
	}

	gitVersionDescriptor := git.GitVersionDescriptor{
		Version:     &version,
		VersionType: &versionType,
	}

	reader, err := v.GitClient.GetItemContent(ctx, git.GetItemContentArgs{
		RepositoryId:      &repositoryID,
		Project:           &ProjectID,
		Path:              &path,
		VersionDescriptor: &gitVersionDescriptor,
	})
	if err != nil {
		return "", err
	}
	defer reader.Close()

	// Read the content from the reader returned by GetBlobContent
	content, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

// GetTektonDir implements provider.Interface.
func (v *Provider) GetTektonDir(ctx context.Context, runevent *info.Event, path, provenance string) (string, error) {
	repositoryID := runevent.RepositoryID
	ProjectID := runevent.ProjectID
	var version string
	var versionType git.GitVersionType

	if provenance == "default_branch" {
		version = runevent.DefaultBranch
		versionType = git.GitVersionTypeValues.Branch
		v.Logger.Infof("Using Tekton definition from default branch: %s", version)
	} else {
		version = runevent.SHA
		versionType = git.GitVersionTypeValues.Commit
		v.Logger.Infof("Using Tekton definition from commit ID: %s", version)
	}

	gitVersionDescriptor := git.GitVersionDescriptor{
		Version:     &version,
		VersionType: &versionType,
	}

	// Check if the path exists and is a directory
	item, err := v.GitClient.GetItem(ctx, git.GetItemArgs{
		RepositoryId:      &repositoryID,
		Project:           &ProjectID,
		Path:              &path,
		VersionDescriptor: &gitVersionDescriptor,
	})
	if err != nil {
		return "", fmt.Errorf("failed to fetch the item: %w", err)
	}

	if item == nil {
		return "", fmt.Errorf("no item found under the specified path '%s' in the repository at the given branch or commit '%s'", path, version)
	}

	// Get the SHA of the directory and fetch the tree
	tree, err := v.GitClient.GetTree(ctx, git.GetTreeArgs{
		RepositoryId: &repositoryID,
		Project:      &ProjectID,
		Sha1:         item.ObjectId,
		Recursive:    toBoolPtr(true),
	})
	if err != nil {
		return "", fmt.Errorf("failed to fetch the tree: %w", err)
	}

	// Concatenate all YAML files found within the tree entries
	result, err := v.concatAllYamlFiles(ctx, tree.TreeEntries, repositoryID, ProjectID)
	if err != nil {
		return "", err
	}
	return result, nil
}

func toBoolPtr(b bool) *bool {
	return &b
}

func (v *Provider) concatAllYamlFiles(ctx context.Context, entries *[]git.GitTreeEntryRef, repositoryID, projectID string) (string, error) {
	var allTemplates string

	for _, entry := range *entries {
		if *entry.GitObjectType == git.GitObjectTypeValues.Blob && (strings.HasSuffix(*entry.RelativePath, ".yaml") || strings.HasSuffix(*entry.RelativePath, ".yml")) {
			// Use the object ID (SHA) of the blob to fetch its content
			data, err := v.getObject(ctx, repositoryID, projectID, *entry.ObjectId)
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

func (v *Provider) getObject(ctx context.Context, repositoryID, projectID, sha string) ([]byte, error) {
	reader, err := v.GitClient.GetBlobContent(ctx, git.GetBlobContentArgs{
		RepositoryId: &repositoryID,
		Project:      &projectID,
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

func (v *Provider) SetClient(_ context.Context, run *params.Run, event *info.Event, _ *v1alpha1.Repository, _ *events.EventEmitter) error {
	var err error

	if event.Provider.Token == "" {
		return fmt.Errorf("no git_provider.secret has been set in the repo crd")
	}

	organizationURL := event.Organization
	connection := azuredevops.NewPatConnection(organizationURL, event.Provider.Token)
	ctx := context.Background()
	v.GitClient, err = git.NewClient(ctx, connection)
	if err != nil {
		return err
	}
	v.CoreClient, err = core.NewClient(ctx, connection)
	if err != nil {
		return err
	}
	v.Token = &event.Provider.Token
	v.run = run
	v.ctx = ctx
	v.connection = connection

	return nil
}

func (v *Provider) SetLogger(logger *zap.SugaredLogger) {
	v.Logger = logger
}

func (v *Provider) Validate(context.Context, *params.Run, *info.Event) error {
	return nil
}
