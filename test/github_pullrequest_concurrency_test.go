//go:build e2e
// +build e2e

package test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	tgithub "github.com/openshift-pipelines/pipelines-as-code/test/pkg/github"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/payload"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	"github.com/tektoncd/pipeline/pkg/names"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGithubPullRequestConcurrency(t *testing.T) {
	ctx := context.Background()
	label := "Github PullRequest Concurrent"

	targetNS := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-ns")
	runcnx, opts, ghcnx, err := tgithub.Setup(ctx, false)
	assert.NilError(t, err)

	logmsg := fmt.Sprintf("Testing %s with Github APPS integration on %s", label, targetNS)
	runcnx.Clients.Log.Info(logmsg)

	repoinfo, resp, err := ghcnx.Client.Repositories.Get(ctx, opts.Organization, opts.Repo)
	assert.NilError(t, err)
	if resp != nil && resp.Response.StatusCode == http.StatusNotFound {
		t.Errorf("Repository %s not found in %s", opts.Organization, opts.Repo)
	}

	// set concurrency
	opts.Concurrency = 1

	err = tgithub.CreateCRD(ctx, t, repoinfo, runcnx, opts, targetNS)
	assert.NilError(t, err)

	yamlFiles := map[string]string{".tekton/pr_longrunning.yaml": "testdata/pipelinerun_long_running.yaml", ".tekton/pr_longrunninganother": "testdata/pipelinerun_long_running_another.yaml"}
	entries, err := payload.GetEntries(yamlFiles, targetNS, options.MainBranch, options.PullRequestEvent)
	assert.NilError(t, err)

	targetRefName := fmt.Sprintf("refs/heads/%s",
		names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-test"))

	sha, err := tgithub.PushFilesToRef(ctx, ghcnx.Client, logmsg, repoinfo.GetDefaultBranch(), targetRefName,
		opts.Organization, opts.Repo, entries)
	assert.NilError(t, err)
	runcnx.Clients.Log.Infof("Commit %s has been created and pushed to %s", sha, targetRefName)

	prNumber, err := tgithub.PRCreate(ctx, runcnx, ghcnx, opts.Organization,
		opts.Repo, targetRefName, repoinfo.GetDefaultBranch(), logmsg)
	assert.NilError(t, err)

	runcnx.Clients.Log.Info("waiting to let controller process the event")
	time.Sleep(5 * time.Second)

	allPR, err := runcnx.Clients.Tekton.TektonV1beta1().PipelineRuns(targetNS).List(ctx, metav1.ListOptions{})
	assert.NilError(t, err)
	assert.Equal(t, len(allPR.Items), 2)

	var pending, running bool
	for _, pr := range allPR.Items {
		if pr.Spec.Status == "" {
			running = true
		} else {
			pending = true
		}
	}
	if !(pending && running) {
		runcnx.Clients.Log.Fatal("one pipelineRun must be in running state and one in pending")
	}

	wait.Succeeded(ctx, t, runcnx, opts, options.PullRequestEvent, targetNS, len(yamlFiles), sha, logmsg)

	runcnx.Clients.Log.Infof("Waiting for Repository to be updated for second pipelineRun")
	waitOpts := wait.Opts{
		RepoName:        targetNS,
		Namespace:       targetNS,
		MinNumberStatus: 1,
		PollTimeout:     wait.DefaultTimeout,
		TargetSHA:       sha,
	}
	err = wait.UntilRepositoryUpdated(ctx, runcnx.Clients, waitOpts)
	assert.NilError(t, err)

	defer tgithub.TearDown(ctx, t, runcnx, ghcnx, prNumber, targetRefName, targetNS, opts)
}
