//go:build e2e
// +build e2e

package test

import (
	"context"
	"testing"

	tbb "github.com/openshift-pipelines/pipelines-as-code/test/pkg/bitbucketcloud"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	"github.com/tektoncd/pipeline/pkg/names"
	"gotest.tools/v3/assert"
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

	pr, repobranch := tbb.MakePR(t, bprovider, runcnx, bcrepo, opts, title, targetNS, targetRefName)
	defer tbb.TearDown(ctx, t, runcnx, bprovider, opts, pr.ID, targetRefName, targetNS)

	hash, ok := repobranch.Target["hash"].(string)
	assert.Assert(t, ok)

	sopt := wait.SuccessOpt{
		TargetNS:        targetNS,
		OnEvent:         options.PullRequestEvent,
		NumberofPRMatch: 1,
		SHA:             hash,
		Title:           title,
		MinNumberStatus: 1,
	}
	wait.Succeeded(ctx, t, runcnx, opts, sopt)
}

// Local Variables:
// compile-command: "go test -tags=e2e -v -run TestBitbucketCloudPullRequest$ ."
// End:
