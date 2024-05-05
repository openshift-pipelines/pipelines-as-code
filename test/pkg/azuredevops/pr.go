package azuredevops

import (
	"context"
	"fmt"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
	azprovider "github.com/openshift-pipelines/pipelines-as-code/pkg/provider/azuredevops"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/payload"
	"gotest.tools/v3/assert"
)

func MakePR(ctx context.Context, t *testing.T, azProvider azprovider.Provider, opts options.E2E, title, targetNS string, targetRefName string) (*int, *string, *string) {
	commitAuthor := "Azure DevOps Pipelines E2E test"
	commitEmail := "e2e-pipelines@azure.com"

	entries, err := payload.GetEntries(
		map[string]string{".tekton/pipelinerun.yaml": "testdata/pipelinerun.yaml"},
		targetNS, options.MainBranch, triggertype.PullRequest.String(), map[string]string{})
	assert.NilError(t, err)
	fileContent := entries[".tekton/pipelinerun.yaml"]

	gitClient := azProvider.Client
	filter := "heads/main"

	// Create a new branch from the default branch
	defaultBranchRef, err := gitClient.GetRefs(ctx, git.GetRefsArgs{
		RepositoryId: &opts.ProjectName,
		Project:      &opts.ProjectName,
		Filter:       &filter,
	})
	assert.NilError(t, err)

	defaultBranch := defaultBranchRef.Value[0]

	fullTargetRN := fmt.Sprintf("refs/heads/%s", targetRefName)

	newRef := &git.GitRefUpdate{
		Name:        &fullTargetRN,
		OldObjectId: defaultBranch.ObjectId,
		NewObjectId: defaultBranch.ObjectId,
	}
	refUpdates := []git.GitRefUpdate{*newRef}
	_, err = gitClient.UpdateRefs(ctx, git.UpdateRefsArgs{
		RefUpdates:   &refUpdates,
		RepositoryId: &opts.ProjectName,
		Project:      &opts.ProjectName,
	})
	assert.NilError(t, err)

	//Comment := fmt.Sprintf("Initial commit of PipelineRun to %s", targetRefName)
	Path := ".tekton/pipelinerun.yaml"

	changes := []git.GitChange{
		{
			ChangeType: &git.VersionControlChangeTypeValues.Add,
			Item: git.GitItem{
				Path: &Path,
			},
			NewContent: &git.ItemContent{
				Content:     &fileContent,
				ContentType: &git.ItemContentTypeValues.RawText,
			},
		},
	}

	interfaceChanges := make([]interface{}, len(changes))
	for i, change := range changes {
		interfaceChanges[i] = change
	}

	commits := []git.GitCommitRef{
		{
			Comment: &title,
			Changes: &interfaceChanges,
			Author: &git.GitUserDate{
				Email: &commitEmail,
				Name:  &commitAuthor,
			},
			Committer: &git.GitUserDate{
				Email: &commitEmail,
				Name:  &commitAuthor,
			},
		},
	}

	push := git.GitPush{
		RefUpdates: &[]git.GitRefUpdate{
			{
				Name:        &fullTargetRN,
				OldObjectId: newRef.NewObjectId,
			},
		},
		Commits: &commits,
	}
	pushResponse, err := gitClient.CreatePush(ctx, git.CreatePushArgs{
		Push:         &push,
		RepositoryId: &opts.ProjectName,
		Project:      &opts.ProjectName,
	})
	assert.NilError(t, err)

	// Create a pull request
	MainRefName := "refs/heads/main"
	Description := "A new PR for e2e testing"
	pr, err := gitClient.CreatePullRequest(ctx, git.CreatePullRequestArgs{
		GitPullRequestToCreate: &git.GitPullRequest{
			SourceRefName: &fullTargetRN,
			TargetRefName: &MainRefName,
			Title:         &title,
			Description:   &Description,
		},
		RepositoryId: &opts.ProjectName,
		Project:      &opts.ProjectName,
	})
	assert.NilError(t, err)
	return pr.PullRequestId, newRef.Name, (*pushResponse.Commits)[0].CommitId

}
