//go:build e2e
// +build e2e

package test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/sort"
	tgithub "github.com/openshift-pipelines/pipelines-as-code/test/pkg/github"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/payload"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/names"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGithubSecondPullRequestConcurrency1by1(t *testing.T) {
	ctx := context.Background()
	label := "Github PullRequest Concurrent, sequentially one by one"
	numberOfPipelineRuns := 5
	maxNumberOfConcurrentPipelineRuns := 1
	testGithubConcurrency(ctx, t, maxNumberOfConcurrentPipelineRuns, numberOfPipelineRuns, label, true, map[string]string{})
}

func TestGithubSecondPullRequestConcurrency3by3(t *testing.T) {
	ctx := context.Background()
	label := "Github PullRequest Concurrent three at time"
	numberOfPipelineRuns := 10
	maxNumberOfConcurrentPipelineRuns := 3
	testGithubConcurrency(ctx, t, maxNumberOfConcurrentPipelineRuns, numberOfPipelineRuns, label, false, map[string]string{})
}

// TestGithubSecondPullRequestConcurrency1by1WithError will test concurrency and an error, this used to crash for us previously.
func TestGithubSecondPullRequestConcurrency1by1WithError(t *testing.T) {
	ctx := context.Background()
	label := "Github PullRequest Concurrent, sequentially one by one with one bad apple"
	numberOfPipelineRuns := 1
	maxNumberOfConcurrentPipelineRuns := 1
	testGithubConcurrency(ctx, t, maxNumberOfConcurrentPipelineRuns, numberOfPipelineRuns, label, false, map[string]string{
		".tekton/00-bad-apple.yaml": "testdata/failures/bad-runafter-task.yaml",
	})
}

func testGithubConcurrency(ctx context.Context, t *testing.T, maxNumberOfConcurrentPipelineRuns, numberOfPipelineRuns int, label string, checkOrdering bool, yamlFiles map[string]string) {
	pipelineRunFileNamePrefix := "prlongrunnning-"
	targetNS := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-ns")
	_, runcnx, opts, ghcnx, err := tgithub.Setup(ctx, true, false)
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

	for i := 1; i <= numberOfPipelineRuns; i++ {
		yamlFiles[fmt.Sprintf(".tekton/%s%d.yaml", pipelineRunFileNamePrefix, i)] = "testdata/pipelinerun_long_running.yaml"
	}

	entries, err := payload.GetEntries(yamlFiles, targetNS, options.MainBranch, "pull_request", map[string]string{})
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
	maxLoop := 30
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
		runcnx.Clients.Log.Infof("number of unsuccessful PR %d out of %d, waiting 10s more in the waiting loop: %d/%d", unsuccessful, numberOfPipelineRuns, i, maxLoop)
		// it's high because it takes time to process on kind
		time.Sleep(10 * time.Second)
	}
	if !finished {
		t.Errorf("the %d pipelineruns has not successfully finished, some of them are still pending or it's abnormally slow to process the Q", numberOfPipelineRuns)
	}

	// sort all the PR by when they have started
	if checkOrdering {
		prs, err := runcnx.Clients.Tekton.TektonV1().PipelineRuns(targetNS).List(ctx, metav1.ListOptions{})
		assert.NilError(t, err)
		sort.PipelineRunSortByStartTime(prs.Items)
		for i := 0; i < numberOfPipelineRuns; i++ {
			prExpectedName := fmt.Sprintf("%s%d", pipelineRunFileNamePrefix, len(prs.Items)-i)
			prActualName := prs.Items[i].GetName()
			assert.Assert(t, strings.HasPrefix(prActualName, prExpectedName), "prActualName: %s does not start with expected prefix %s, was is ordered properly at start time", prActualName, prExpectedName)
		}
	}
}
