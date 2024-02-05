//go:build e2e
// +build e2e

package test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/go-github/v56/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
	tgithub "github.com/openshift-pipelines/pipelines-as-code/test/pkg/github"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/payload"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	"github.com/tektoncd/pipeline/pkg/names"
	"gotest.tools/v3/assert"
)

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func TestGithubPullRequestConcurrency(t *testing.T) {
	if os.Getenv("NIGHTLY_E2E_TEST") != "true" {
		t.Skip("Skipping test since only enabled for nightly")
	}

	ctx := context.Background()
	label := "Github PullRequest Concurrent"
	numberOfPipelineRuns := 10
	maxNumberOfConcurrentPipelineRuns := 3

	targetNS := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-ns")
	_, runcnx, opts, ghcnx, err := tgithub.Setup(ctx, false, false)
	assert.NilError(t, err)

	logmsg := fmt.Sprintf("Testing %s with Github APPS integration on %s", label, targetNS)
	runcnx.Clients.Log.Info(logmsg)

	repoinfo, resp, err := ghcnx.Client.Repositories.Get(ctx, opts.Organization, opts.Repo)
	assert.NilError(t, err)
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		t.Errorf("Repository %s not found in %s", opts.Organization, opts.Repo)
	}

	// set concurrency
	opts.Concurrency = maxNumberOfConcurrentPipelineRuns

	err = tgithub.CreateCRD(ctx, t, repoinfo, runcnx, opts, targetNS)
	assert.NilError(t, err)

	yamlFiles := map[string]string{}
	for i := 1; i <= numberOfPipelineRuns; i++ {
		yamlFiles[fmt.Sprintf(".tekton/prlongrunnning-%d.yaml", i)] = "testdata/pipelinerun_long_running.yaml"
	}

	entries, err := payload.GetEntries(yamlFiles, targetNS, options.MainBranch, triggertype.PullRequest.String(), map[string]string{})
	assert.NilError(t, err)

	targetRefName := fmt.Sprintf("refs/heads/%s",
		names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-test"))

	sha, vref, err := tgithub.PushFilesToRef(ctx, ghcnx.Client, logmsg, repoinfo.GetDefaultBranch(), targetRefName,
		opts.Organization, opts.Repo, entries)
	assert.NilError(t, err)
	runcnx.Clients.Log.Infof("Commit %s has been created and pushed to %s", sha, vref.GetURL())

	prNumber, err := tgithub.PRCreate(ctx, runcnx, ghcnx, opts.Organization,
		opts.Repo, targetRefName, repoinfo.GetDefaultBranch(), logmsg)
	assert.NilError(t, err)
	defer tgithub.TearDown(ctx, t, runcnx, ghcnx, prNumber, targetRefName, targetNS, opts)

	runcnx.Clients.Log.Info("waiting to let controller process the event")
	time.Sleep(5 * time.Second)

	waitOpts := wait.Opts{
		RepoName:        targetNS,
		Namespace:       targetNS,
		MinNumberStatus: 1,
		PollTimeout:     wait.DefaultTimeout,
		TargetSHA:       sha,
	}
	assert.NilError(t, wait.UntilMinPRAppeared(ctx, runcnx.Clients, waitOpts, numberOfPipelineRuns))

	runningChecks := 0
	finishedArray := []string{}
	for i := 0; i < 15; i++ {
		checkruns, _, err := ghcnx.Client.Checks.ListCheckRunsForRef(ctx, opts.Organization, opts.Repo, targetRefName, &github.ListCheckRunsOptions{})
		assert.NilError(t, err)
		if checkruns.Total == nil || *checkruns.Total < numberOfPipelineRuns {
			t.Logf("Waiting for all checks to be created: %d/%d", *checkruns.Total, numberOfPipelineRuns)
			time.Sleep(5 * time.Second)
			continue
		}
		assert.Assert(t, *checkruns.Total >= numberOfPipelineRuns)
		for _, checkrun := range checkruns.CheckRuns {
			cname := checkrun.GetName()
			switch {
			case checkrun.GetStatus() != "completed" && checkrun.GetConclusion() != "success":
				runcnx.Clients.Log.Infof("Waiting for CheckRun %s to be completed", cname)
			case checkrun.GetStatus() == "running":
				runningChecks++
			default:
				// check if cname is in finishedArray
				if !contains(finishedArray, cname) {
					runcnx.Clients.Log.Infof("PipelineRun %s has finished %d/%d",
						cname, len(finishedArray), numberOfPipelineRuns)
					finishedArray = append(finishedArray, cname)
				}
			}
		}
		if len(finishedArray) == numberOfPipelineRuns {
			break
		}
		if runningChecks > maxNumberOfConcurrentPipelineRuns {
			runcnx.Clients.Log.Fatalf("Too many running checks %d our maxAmountOfConcurrentPRS == %d",
				runningChecks, numberOfPipelineRuns)
		}
		// it's high so we limit our ratelimitation
		time.Sleep(10 * time.Second)
	}
}
