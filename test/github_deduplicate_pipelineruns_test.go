//go:build e2e

package test

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"testing"

	tgithub "github.com/openshift-pipelines/pipelines-as-code/test/pkg/github"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/payload"
	twait "github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	"github.com/tektoncd/pipeline/pkg/names"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGithubDeduplicatePipelineRuns(t *testing.T) {
	targetNS := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-ns")

	ctx := context.TODO()
	ctx, runcnx, opts, ghcnx, err := tgithub.Setup(ctx, false, false)
	assert.NilError(t, err)

	yamlEntries := map[string]string{}
	yamlEntries[".tekton/pipelinerun-match-all-branch.yaml"] = "testdata/pipelinerun.yaml"

	repoinfo, resp, err := ghcnx.Client().Repositories.Get(ctx, opts.Organization, opts.Repo)
	assert.NilError(t, err)
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		t.Errorf("Repository %s not found in %s", opts.Organization, opts.Repo)
	}

	err = tgithub.CreateCRD(ctx, t, repoinfo, runcnx, opts, targetNS)
	assert.NilError(t, err)

	entries, err := payload.GetEntries(yamlEntries, targetNS, "*", "pull_request, push", map[string]string{})
	assert.NilError(t, err)

	targetRefName := fmt.Sprintf("refs/heads/%s",
		names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-test"))

	sha, vref, err := tgithub.PushFilesToRef(ctx, ghcnx.Client(), "Test Github Deduplicate PipelineRuns", repoinfo.GetDefaultBranch(), targetRefName,
		opts.Organization, opts.Repo, entries)
	assert.NilError(t, err)

	runcnx.Clients.Log.Infof("Commit %s has been created and pushed to %s", sha, vref.GetURL())
	number, err := tgithub.PRCreate(ctx, runcnx, ghcnx, opts.Organization,
		opts.Repo, targetRefName, repoinfo.GetDefaultBranch(), "Test Github Deduplicate PipelineRuns")
	assert.NilError(t, err)

	g := &tgithub.PRTest{
		PRNumber:        number,
		SHA:             sha,
		TargetNamespace: targetNS,
		Logger:          runcnx.Clients.Log,
		Provider:        ghcnx,
		Options:         opts,
		TargetRefName:   targetRefName,
		Cnx:             runcnx,
	}

	defer g.TearDown(ctx, t)

	runcnx.Clients.Log.Infof("Pull request %d has been created", number)
	err = twait.UntilPipelineRunCreated(ctx, runcnx.Clients, twait.Opts{
		RepoName:        targetNS,
		Namespace:       targetNS,
		MinNumberStatus: 1,
		PollTimeout:     twait.DefaultTimeout,
		TargetSHA:       sha,
	})
	assert.NilError(t, err)

	// but here we can't guarantee that for which event PipelineRun is created because we don't which
	// event is received first on controller either push or pull_request.
	prs, err := runcnx.Clients.Tekton.TektonV1().PipelineRuns(targetNS).List(ctx, metav1.ListOptions{})
	assert.NilError(t, err)
	assert.Assert(t, len(prs.Items) == 1)

	maxLines := int64(50)
	err = twait.RegexpMatchingInControllerLog(ctx, runcnx, *regexp.MustCompile("Skipping duplicate PipelineRun"), 10, "controller", &maxLines)
	assert.NilError(t, err)
}
