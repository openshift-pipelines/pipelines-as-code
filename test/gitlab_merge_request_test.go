//go:build e2e
// +build e2e

package test

import (
	"context"
	"net/http"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/cctx"
	tgitlab "github.com/openshift-pipelines/pipelines-as-code/test/pkg/gitlab"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/payload"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/scm"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	"github.com/tektoncd/pipeline/pkg/names"
	clientGitlab "github.com/xanzy/go-gitlab"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGitlabMergeRequest(t *testing.T) {
	targetNS := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-ns")
	ctx := context.Background()
	runcnx, opts, glprovider, err := tgitlab.Setup(ctx)
	assert.NilError(t, err)
	ctx, err = cctx.GetControllerCtxInfo(ctx, runcnx)
	assert.NilError(t, err)
	runcnx.Clients.Log.Info("Testing with Gitlab")

	projectinfo, resp, err := glprovider.Client.Projects.GetProject(opts.ProjectID, nil)
	assert.NilError(t, err)
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		t.Errorf("Repository %s not found in %s", opts.Organization, opts.Repo)
	}

	err = tgitlab.CreateCRD(ctx, projectinfo, runcnx, targetNS, nil)
	assert.NilError(t, err)

	entries, err := payload.GetEntries(map[string]string{
		".tekton/pipelinerun.yaml":       "testdata/pipelinerun.yaml",
		".tekton/pipelinerun-clone.yaml": "testdata/pipelinerun-clone.yaml",
	}, targetNS, projectinfo.DefaultBranch,
		triggertype.PullRequest.String(), map[string]string{})
	assert.NilError(t, err)

	targetRefName := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-test")

	gitCloneURL, err := scm.MakeGitCloneURL(projectinfo.WebURL, opts.UserName, opts.Password)
	assert.NilError(t, err)
	scmOpts := &scm.Opts{
		GitURL:        gitCloneURL,
		Log:           runcnx.Clients.Log,
		WebURL:        projectinfo.WebURL,
		TargetRefName: targetRefName,
		BaseRefName:   projectinfo.DefaultBranch,
	}
	scm.PushFilesToRefGit(t, scmOpts, entries)

	runcnx.Clients.Log.Infof("Branch %s has been created and pushed with files", targetRefName)
	mrTitle := "TestMergeRequest - " + targetRefName
	mrID, err := tgitlab.CreateMR(glprovider.Client, opts.ProjectID, targetRefName, projectinfo.DefaultBranch, mrTitle)
	assert.NilError(t, err)
	runcnx.Clients.Log.Infof("MergeRequest %s/-/merge_requests/%d has been created", projectinfo.WebURL, mrID)
	defer tgitlab.TearDown(ctx, t, runcnx, glprovider, mrID, targetRefName, targetNS, opts.ProjectID)

	// updating labels to test if we skip them, this used to create multiple PRs
	_, _, err = glprovider.Client.MergeRequests.UpdateMergeRequest(opts.ProjectID, mrID, &clientGitlab.UpdateMergeRequestOptions{
		Labels: &clientGitlab.Labels{"hello-label"},
	})
	assert.NilError(t, err)

	// Send another Push to make an update and make sure we react to it
	// this used to be not working
	entries, err = payload.GetEntries(map[string]string{
		"hello-world.yaml": "testdata/pipelinerun.yaml",
	}, targetNS, projectinfo.DefaultBranch,
		triggertype.PullRequest.String(), map[string]string{})
	assert.NilError(t, err)
	scmOpts.BaseRefName = targetRefName
	scm.PushFilesToRefGit(t, scmOpts, entries)

	sopt := wait.SuccessOpt{
		Title:           mrTitle,
		OnEvent:         "Merge Request",
		TargetNS:        targetNS,
		NumberofPRMatch: 4,
		SHA:             "",
	}
	wait.Succeeded(ctx, t, runcnx, opts, sopt)
	prsNew, err := runcnx.Clients.Tekton.TektonV1().PipelineRuns(targetNS).List(ctx, metav1.ListOptions{})
	assert.NilError(t, err)
	assert.Assert(t, len(prsNew.Items) == 4)

	for i := 0; i < len(prsNew.Items); i++ {
		assert.Equal(t, "Merge Request", prsNew.Items[i].Annotations[keys.EventType])
	}
}

// Local Variables:
// compile-command: "go test -tags=e2e -v -run ^TestGitlabMergeRequest$"
// End:
