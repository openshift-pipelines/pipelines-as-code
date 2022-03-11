//go:build e2e
// +build e2e

package test

import (
	"context"
	"net/http"
	"os"
	"testing"

	pacv1alpha1 "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/gitlab"
	tgitlab "github.com/openshift-pipelines/pipelines-as-code/test/pkg/gitlab"
	trepo "github.com/openshift-pipelines/pipelines-as-code/test/pkg/repository"
	"github.com/tektoncd/pipeline/pkg/names"
	ghlib "github.com/xanzy/go-gitlab"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGitlabMergeRequest(t *testing.T) {
	targetNS := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-ns")
	ctx := context.Background()
	runcnx, opts, glprovider, err := gitlabSetup(ctx)
	assert.NilError(t, err)
	runcnx.Clients.Log.Info("Testing with Gitlab")

	projectinfo, resp, err := glprovider.Client.Projects.GetProject(opts.ProjectID, nil)
	assert.NilError(t, err)
	if resp != nil && resp.Response.StatusCode == http.StatusNotFound {
		t.Errorf("Repository %s not found in %s", opts.Organization, opts.Repo)
	}

	err = createGitlabRepoCRD(ctx, projectinfo, runcnx, targetNS)
	assert.NilError(t, err)

	entries, err := getEntries("testdata/pipelinerun.yaml", targetNS, projectinfo.DefaultBranch, pullRequestEvent)
	assert.NilError(t, err)

	targetRefName := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-test")
	title := "TestMergeRequest - " + targetRefName
	err = tgitlab.PushFilesToRef(glprovider.Client, title,
		projectinfo.DefaultBranch,
		targetRefName,
		opts.ProjectID,
		entries)
	assert.NilError(t, err)
	runcnx.Clients.Log.Infof("Branch %s has been created and pushed with files", targetRefName)
	mrID, err := tgitlab.CreateMR(glprovider.Client, opts.ProjectID, targetRefName, projectinfo.DefaultBranch, title)
	assert.NilError(t, err)
	runcnx.Clients.Log.Infof("MergeRequest %s/-/merge_requests/%d has been created", projectinfo.WebURL, mrID)
	defer gltearDown(ctx, t, runcnx, glprovider, mrID, targetRefName, targetNS, opts.ProjectID)
	checkSuccess(ctx, t, runcnx, opts, "Merge_Request", targetNS, "", title)
}

func createGitlabRepoCRD(ctx context.Context, projectinfo *ghlib.Project, run *params.Run, targetNS string) error {
	err := trepo.CreateNS(ctx, targetNS, run)
	if err != nil {
		return err
	}

	token, _ := os.LookupEnv("TEST_GITLAB_TOKEN")
	apiURL, _ := os.LookupEnv("TEST_GITLAB_API_URL")
	err = createSecret(ctx, run, map[string]string{"token": token}, targetNS, "webhook-token")
	if err != nil {
		return err
	}
	repository := &pacv1alpha1.Repository{
		ObjectMeta: metav1.ObjectMeta{
			Name: targetNS,
		},
		Spec: pacv1alpha1.RepositorySpec{
			URL: projectinfo.WebURL,
			GitProvider: &pacv1alpha1.GitProvider{
				URL:    apiURL,
				Secret: &pacv1alpha1.GitProviderSecret{Name: "webhook-token", Key: "token"},
			},
		},
	}

	return trepo.CreateRepo(ctx, targetNS, run, repository)
}

func gltearDown(ctx context.Context, t *testing.T, runcnx *params.Run, glprovider gitlab.Provider, mrNumber int, ref string, targetNS string, projectid int) {
	runcnx.Clients.Log.Infof("Closing PR %d", mrNumber)
	if mrNumber != -1 {
		_, _, err := glprovider.Client.MergeRequests.UpdateMergeRequest(projectid, mrNumber,
			&ghlib.UpdateMergeRequestOptions{StateEvent: ghlib.String("close")})
		if err != nil {
			t.Fatal(err)
		}
	}
	nsTearDown(ctx, t, runcnx, targetNS)
	runcnx.Clients.Log.Infof("Deleting Ref %s", ref)
	_, err := glprovider.Client.Branches.DeleteBranch(projectid, ref)
	assert.NilError(t, err)
}

// Local Variables:
// compile-command: "go test -tags=e2e -v -run TestGitlabMergeRequest$ ."
// End:
