//go:build e2e
// +build e2e

package test

import (
	"context"
	"net/http"
	"testing"

	tgitlab "github.com/openshift-pipelines/pipelines-as-code/test/pkg/gitlab"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/payload"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	"github.com/tektoncd/pipeline/pkg/names"
	"gotest.tools/v3/assert"
)

func TestGitlabMergeRequest(t *testing.T) {
	t.Skip()
	targetNS := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-ns")
	ctx := context.Background()
	runcnx, opts, glprovider, err := tgitlab.Setup(ctx)
	assert.NilError(t, err)
	runcnx.Clients.Log.Info("Testing with Gitlab")

	projectinfo, resp, err := glprovider.Client.Projects.GetProject(opts.ProjectID, nil)
	assert.NilError(t, err)
	if resp != nil && resp.Response.StatusCode == http.StatusNotFound {
		t.Errorf("Repository %s not found in %s", opts.Organization, opts.Repo)
	}

	err = tgitlab.CreateCRD(ctx, projectinfo, runcnx, targetNS)
	assert.NilError(t, err)

	entries, err := payload.GetEntries("testdata/pipelinerun.yaml", targetNS, projectinfo.DefaultBranch, options.PullRequestEvent)
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
	defer tgitlab.TearDown(ctx, t, runcnx, glprovider, mrID, targetRefName, targetNS, opts.ProjectID)
	wait.Succeeded(ctx, t, runcnx, opts, "Merge_Request", targetNS, "", title)
}

// Local Variables:
// compile-command: "go test -tags=e2e -v -run TestGitlabMergeRequest$ ."
// End:
