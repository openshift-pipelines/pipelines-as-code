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
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/names"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGithubPullRequestConcurrency(t *testing.T) {
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

	entries, err := payload.GetEntries(yamlFiles, targetNS, options.MainBranch, options.PullRequestEvent, map[string]string{})
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

	finished := false
	maxLoop := 15
	for i := 0; i < maxLoop; i++ {
		unsuccessful := 0
		prs, err := runcnx.Clients.Tekton.TektonV1().PipelineRuns(targetNS).List(ctx, metav1.ListOptions{})
		assert.NilError(t, err)
		for _, pr := range prs.Items {
			if pr.Status.GetConditions() == nil {
				unsuccessful++
				continue
			}
			for _, condition := range pr.Status.GetConditions() {
				if condition.Status == "Unknown" || condition.GetReason() == tektonv1.PipelineRunSpecStatusPending {
					unsuccessful++
					continue
				}
			}
		}
		if unsuccessful == 0 {
			runcnx.Clients.Log.Infof("the %d pipelineruns has successfully finished", numberOfPipelineRuns)
			finished = true
			break
		}
		runcnx.Clients.Log.Infof("number of unsuccessful PR %d out of %d, waiting 10s more, %d/%d", unsuccessful, numberOfPipelineRuns*2, i, maxLoop)
		// it's high because it takes time to process on kind
		time.Sleep(15 * time.Second)
	}
	if !finished {
		t.Errorf("the %d pipelineruns has not successfully finished, some of them are still pending or it's abnormally slow to process the Q", numberOfPipelineRuns)
	}
}
