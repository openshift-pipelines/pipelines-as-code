//go:build e2e
// +build e2e

package test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/ktrysmt/go-bitbucket"
	"github.com/mitchellh/mapstructure"
	pacv1alpha1 "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs/bitbucketcloud"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs/bitbucketcloud/types"
	trepo "github.com/openshift-pipelines/pipelines-as-code/test/pkg/repository"
	"github.com/tektoncd/pipeline/pkg/names"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBitbucketCloudPullRequest(t *testing.T) {
	targetNS := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-ns")
	ctx := context.Background()

	runcnx, opts, bcvcs, err := bitbucketCloudSetup(ctx)
	if err != nil {
		t.Skip(err.Error())
		return
	}
	bcrepo := createBitbucketRepoCRD(ctx, t, bcvcs, runcnx, opts, targetNS)
	targetRefName := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-test")
	title := "TestPullRequest - " + targetRefName

	pr, repobranch := createPR(t, bcvcs, runcnx, bcrepo, opts, title, targetNS, targetRefName)
	defer bitbucketTearDown(ctx, t, runcnx, bcvcs, opts, pr.ID, targetRefName, targetNS)
	checkSuccess(ctx, t, runcnx, opts, pullRequestEvent, targetNS, repobranch.Target["hash"].(string), title)
}

func createPR(t *testing.T, bcvcs bitbucketcloud.VCS, runcnx *params.Run, bcrepo *bitbucket.Repository, opts E2EOptions, title, targetNS, targetRefName string) (*types.PullRequest, *bitbucket.RepositoryBranch) {
	commitAuthor := "OpenShift Pipelines E2E test"
	commitEmail := "e2e-pipelines@redhat.com"

	entries, err := getEntries("testdata/pipelinerun.yaml", targetNS, mainBranch, pullRequestEvent)
	assert.NilError(t, err)
	tmpfile := fs.NewFile(t, "pipelinerun", fs.WithContent(entries[".tekton/pr.yaml"]))
	defer tmpfile.Remove()

	err = bcvcs.Client.Workspaces.Repositories.Repository.WriteFileBlob(&bitbucket.RepositoryBlobWriteOptions{
		Owner:    opts.Owner,
		RepoSlug: opts.Repo,
		FileName: ".tekton/pr.yaml",
		FilePath: tmpfile.Path(),
		Message:  title,
		Branch:   targetRefName,
		Author:   fmt.Sprintf("%s <%s>", commitAuthor, commitEmail),
	})
	assert.NilError(t, err)
	runcnx.Clients.Log.Infof("Using repo %s branch %s", bcrepo.Full_name, targetRefName)

	repobranch, err := bcvcs.Client.Repositories.Repository.GetBranch(&bitbucket.RepositoryBranchOptions{
		Owner:      opts.Owner,
		RepoSlug:   opts.Repo,
		BranchName: targetRefName,
	})
	assert.NilError(t, err)

	intf, err := bcvcs.Client.Repositories.PullRequests.Create(&bitbucket.PullRequestsOptions{
		Owner:        opts.Owner,
		RepoSlug:     opts.Repo,
		Title:        title,
		Message:      "A new PR for testing",
		SourceBranch: targetRefName,
	})
	assert.NilError(t, err)

	pr := &types.PullRequest{}
	err = mapstructure.Decode(intf, pr)
	assert.NilError(t, err)
	runcnx.Clients.Log.Infof("Created PR %s", pr.Links.HTML.HRef)

	return pr, repobranch
}

func bitbucketTearDown(ctx context.Context, t *testing.T, runcnx *params.Run, bcvcs bitbucketcloud.VCS, opts E2EOptions, prNumber int, ref, targetNS string) {
	runcnx.Clients.Log.Infof("Closing PR #%d", prNumber)
	_, err := bcvcs.Client.Repositories.PullRequests.Decline(&bitbucket.PullRequestsOptions{
		ID:       fmt.Sprintf("%d", prNumber),
		Owner:    opts.Owner,
		RepoSlug: opts.Repo,
	})
	assert.NilError(t, err)
	runcnx.Clients.Log.Infof("Deleting ref %s", ref)
	err = bcvcs.Client.Repositories.Repository.DeleteBranch(
		&bitbucket.RepositoryBranchDeleteOptions{
			Owner:    opts.Owner,
			RepoSlug: opts.Repo,
			RefName:  ref,
		},
	)
	assert.NilError(t, err)

	nsTearDown(ctx, t, runcnx, targetNS)
}

func createBitbucketRepoCRD(ctx context.Context, t *testing.T, bcvcs bitbucketcloud.VCS, run *params.Run, opts E2EOptions, targetNS string) *bitbucket.Repository {
	repo, err := bcvcs.Client.Workspaces.Repositories.Repository.Get(
		&bitbucket.RepositoryOptions{
			Owner:    opts.Owner,
			RepoSlug: opts.Repo,
		})
	assert.NilError(t, err)

	links := &types.Links{}
	err = mapstructure.Decode(repo.Links, links)
	assert.NilError(t, err)
	repository := &pacv1alpha1.Repository{
		ObjectMeta: metav1.ObjectMeta{
			Name: targetNS,
		},
		Spec: pacv1alpha1.RepositorySpec{
			URL: links.HTML.HRef,
		},
	}
	err = trepo.CreateNS(ctx, targetNS, run)
	assert.NilError(t, err)

	token, _ := os.LookupEnv("TEST_BITBUCKET_CLOUD_TOKEN")
	apiURL, _ := os.LookupEnv("TEST_BITBUCKET_CLOUD_API_URL")
	apiUser, _ := os.LookupEnv("TEST_BITBUCKET_CLOUD_USER")
	err = createSecret(ctx, run, map[string]string{"token": token}, targetNS, "webhook-token")
	assert.NilError(t, err)
	repository.Spec.WebvcsAPIURL = apiURL
	repository.Spec.WebvcsAPIUser = apiUser
	repository.Spec.WebvcsAPISecret = &pacv1alpha1.WebvcsSecretSpec{Name: "webhook-token", Key: "token"}

	err = trepo.CreateRepo(ctx, targetNS, run, repository)
	assert.NilError(t, err)

	return repo
}

// Local Variables:
// compile-command: "go test -tags=e2e -v -run TestBitbucketCloudPullRequest$ ."
// End:
