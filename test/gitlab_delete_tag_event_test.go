//go:build e2e

package test

import (
	"context"
	"net/http"
	"regexp"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/cctx"
	tgitlab "github.com/openshift-pipelines/pipelines-as-code/test/pkg/gitlab"
	twait "github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	"github.com/tektoncd/pipeline/pkg/names"
	"gotest.tools/v3/assert"
)

func TestGitlabDeleteTagEvent(t *testing.T) {
	targetNS := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-ns")
	ctx := context.Background()
	runcnx, opts, glprovider, err := tgitlab.Setup(ctx)
	assert.NilError(t, err)
	ctx, err = cctx.GetControllerCtxInfo(ctx, runcnx)
	assert.NilError(t, err)
	runcnx.Clients.Log.Info("Testing with Gitlab")

	projectinfo, resp, err := glprovider.Client().Projects.GetProject(opts.ProjectID, nil)
	assert.NilError(t, err)
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		t.Errorf("Repository %s not found in %s", opts.Organization, opts.Repo)
	}
	defer tgitlab.TearDown(ctx, t, runcnx, glprovider, -1, "", targetNS, opts.ProjectID)

	err = tgitlab.CreateCRD(ctx, projectinfo, runcnx, opts, targetNS, nil)
	assert.NilError(t, err)

	tagName := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("v1.0")
	err = tgitlab.CreateTag(glprovider.Client(), projectinfo.ID, tagName)
	// if something goes wrong in creating tag and tag remains in
	// repository CleanTag will clear that and doesn't throw any error.
	defer tgitlab.CleanTag(glprovider.Client(), projectinfo.ID, tagName)
	assert.NilError(t, err)
	runcnx.Clients.Log.Infof("Created Tag %s in %s repository", tagName, projectinfo.Name)

	err = tgitlab.DeleteTag(glprovider.Client(), projectinfo.ID, tagName)
	assert.NilError(t, err)
	runcnx.Clients.Log.Infof("Deleted Tag %s in %s repository", tagName, projectinfo.Name)

	logLinesToCheck := int64(100)
	reg := regexp.MustCompile("event Delete Tag Push Hook is not supported*")
	err = twait.RegexpMatchingInControllerLog(ctx, runcnx, *reg, 10, "controller", &logLinesToCheck)
	assert.NilError(t, err)
}

// Local Variables:
// compile-command: "go test -tags=e2e -v -run TestGitlabDeleteTagEvent$ ."
// End:
