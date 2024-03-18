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

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
	tgithub "github.com/openshift-pipelines/pipelines-as-code/test/pkg/github"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/payload"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/repository"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/names"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGithubSecondPullRequestConcurrencyMultiplePR(t *testing.T) {
	ctx := context.Background()
	label := "Github Multiple PullRequest Concurrency"
	numberOfPullRequest := 5
	numberOfPipelineRuns := 2
	maxNumberOfConcurrentPipelineRuns := 1

	targetNS := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-ns")
	_, runcnx, opts, ghcnx, err := tgithub.Setup(ctx, true, false)
	assert.NilError(t, err)
	repoinfo, resp, err := ghcnx.Client.Repositories.Get(ctx, opts.Organization, opts.Repo)
	assert.NilError(t, err)
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		t.Errorf("Repository %s not found in %s", opts.Organization, opts.Repo)
	}
	// set concurrency
	opts.Concurrency = maxNumberOfConcurrentPipelineRuns
	err = tgithub.CreateCRD(ctx, t, repoinfo, runcnx, opts, targetNS)
	assert.NilError(t, err)

	for prc := 0; prc < numberOfPullRequest; prc++ {
		branchName := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("branch")
		logmsg := fmt.Sprintf("Testing %s with Github APPS integration branch %s namespace %s", label, branchName, targetNS)
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

		prNumber, err := tgithub.PRCreate(ctx, runcnx, ghcnx, opts.Organization, opts.Repo, targetRefName, repoinfo.GetDefaultBranch(), logmsg)
		assert.NilError(t, err)

		g := tgithub.PRTest{
			Cnx:           runcnx,
			Options:       opts,
			Provider:      ghcnx,
			TargetRefName: targetRefName,
			PRNumber:      prNumber,
			SHA:           sha,
			Logger:        runcnx.Clients.Log,
		}
		defer g.TearDown(ctx, t)
	}

	finished := false
	maxLoop := 30
	allPipelinesRunsCnt := numberOfPullRequest * numberOfPipelineRuns
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
			runcnx.Clients.Log.Infof("the %d pipelineruns has successfully finished", allPipelinesRunsCnt)
			finished = true
			break
		}
		runcnx.Clients.Log.Infof("number of unsuccessful PR %d out of %d, waiting 10s more, %d/%d", unsuccessful, allPipelinesRunsCnt, i, maxLoop)
		// it's high because it takes time to process on kind
		time.Sleep(10 * time.Second)
	}
	if !finished {
		t.Errorf("we didn't get %d pipelineruns as successful, some of them are still pending or it's abnormally slow to process the Q", allPipelinesRunsCnt)
	}

	prs, err := runcnx.Clients.Tekton.TektonV1().PipelineRuns(targetNS).List(ctx, metav1.ListOptions{})
	assert.NilError(t, err)
	assert.Equal(t, len(prs.Items), allPipelinesRunsCnt, "we should have had %d created, we got %d", allPipelinesRunsCnt, len(prs.Items))

	if os.Getenv("TEST_NOCLEANUP") != "true" {
		repository.NSTearDown(ctx, t, runcnx, targetNS)
		return
	}
}
