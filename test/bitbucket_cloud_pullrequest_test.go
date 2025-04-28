//go:build e2e
// +build e2e

package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ktrysmt/go-bitbucket"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
	tbb "github.com/openshift-pipelines/pipelines-as-code/test/pkg/bitbucketcloud"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/payload"
	twait "github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	"github.com/tektoncd/pipeline/pkg/names"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func TestBitbucketCloudPullRequest(t *testing.T) {
	targetNS := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-ns")
	ctx := context.Background()

	runcnx, opts, bprovider, err := tbb.Setup(ctx)
	if err != nil {
		t.Skip(err.Error())
		return
	}
	bcrepo := tbb.CreateCRD(ctx, t, bprovider, runcnx, opts, targetNS)
	targetRefName := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-test")
	title := "TestPullRequest - " + targetRefName

	entries, err := payload.GetEntries(
		map[string]string{".tekton/pipelinerun.yaml": "testdata/pipelinerun.yaml"},
		targetNS, options.MainBranch, triggertype.PullRequest.String(), map[string]string{})
	assert.NilError(t, err)

	pr, repobranch := tbb.MakePR(t, bprovider, runcnx, bcrepo, opts, title, targetRefName, entries)
	defer tbb.TearDown(ctx, t, runcnx, bprovider, opts, pr.ID, targetRefName, targetNS, false)

	hash, ok := repobranch.Target["hash"].(string)
	assert.Assert(t, ok)

	sopt := twait.SuccessOpt{
		TargetNS:        targetNS,
		OnEvent:         triggertype.PullRequest.String(),
		NumberofPRMatch: 1,
		SHA:             hash,
		Title:           title,
		MinNumberStatus: 1,
	}
	twait.Succeeded(ctx, t, runcnx, opts, sopt)
}

func TestBitbucketCloudPullRequestCancelInProgressMerged(t *testing.T) {
	targetNS := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-ns")
	ctx := context.Background()

	runcnx, opts, bprovider, err := tbb.Setup(ctx)
	if err != nil {
		t.Skip(err.Error())
		return
	}
	bcrepo := tbb.CreateCRD(ctx, t, bprovider, runcnx, opts, targetNS)
	targetRefName := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-test")
	title := "TestPullRequest - " + targetRefName

	entries, err := payload.GetEntries(
		map[string]string{".tekton/pipelinerun-cancel-in-progress.yaml": "testdata/pipelinerun-cancel-in-progress.yaml"},
		targetNS, options.MainBranch, triggertype.PullRequest.String(), map[string]string{})
	assert.NilError(t, err)

	pr, repobranch := tbb.MakePR(t, bprovider, runcnx, bcrepo, opts, title, targetRefName, entries)
	defer tbb.TearDown(ctx, t, runcnx, bprovider, opts, pr.ID, targetRefName, targetNS, true)

	sha, ok := repobranch.Target["hash"].(string)
	assert.Assert(t, ok)

	waitOpts := twait.Opts{
		RepoName:        targetNS,
		Namespace:       targetNS,
		MinNumberStatus: 1,
		PollTimeout:     twait.DefaultTimeout,
		TargetSHA:       sha,
	}
	err = twait.UntilPipelineRunCreated(ctx, runcnx.Clients, waitOpts)
	assert.NilError(t, err)

	po := &bitbucket.PullRequestsOptions{
		RepoSlug: opts.Repo,
		Owner:    opts.Organization,
		ID:       fmt.Sprintf("%d", pr.ID),
	}
	_, err = bprovider.Client().Repositories.PullRequests.Decline(po)
	assert.NilError(t, err)

	runcnx.Clients.Log.Info("Waiting 10 seconds to check things has been canceled")
	time.Sleep(10 * time.Second) // “Evil does not sleep. It waits.” - Galadriel

	prs, err := runcnx.Clients.Tekton.TektonV1().PipelineRuns(targetNS).List(context.Background(), metav1.ListOptions{})
	assert.NilError(t, err)
	assert.Equal(t, len(prs.Items), 1, "should have only one pipelinerun, but we have: %d", len(prs.Items))

	assert.Equal(t, prs.Items[0].GetStatusCondition().GetCondition(apis.ConditionSucceeded).GetReason(), "Cancelled", "should have been canceled")
}

// Local Variables:
// compile-command: "go test -tags=e2e -v -run TestBitbucketCloudPullRequest$ ."
// End:
