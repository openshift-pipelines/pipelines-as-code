//go:build e2e
// +build e2e

package test

import (
	"context"
	"fmt"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
	tbbs "github.com/openshift-pipelines/pipelines-as-code/test/pkg/bitbucketserver"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"

	"github.com/tektoncd/pipeline/pkg/names"
	"gotest.tools/v3/assert"
)

func TestBitbucketServerPullRequest(t *testing.T) {
	targetNS := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-ns")
	ctx := context.Background()

	ctx, runcnx, opts, provider, restClient, err := tbbs.Setup(ctx)
	assert.NilError(t, err)

	repo := tbbs.CreateCRD(ctx, t, provider, runcnx, opts, targetNS)
	defer tbbs.TearDownNs(ctx, t, runcnx, targetNS)

	title := "TestPullRequest - " + targetNS
	numberOfFiles := 5
	files := map[string]string{}
	for i := range numberOfFiles {
		files[fmt.Sprintf("pipelinerun-%d.yaml", i)] = "testdata/pipelinerun.yaml"
	}

	pr, commits := tbbs.CreatePR(ctx, t, restClient, *repo, runcnx, opts, files, title, targetNS)
	assert.Assert(t, numberOfFiles == len(commits))
	runcnx.Clients.Log.Infof("Pull Request with title '%s' is created", pr.Title)
	defer tbbs.TearDownBranch(ctx, t, runcnx, provider, restClient, opts, pr.Id, targetNS)

	successOpts := wait.SuccessOpt{
		TargetNS:        targetNS,
		OnEvent:         triggertype.PullRequest.String(),
		NumberofPRMatch: 5,
		Title:           commits[0].Message,
		MinNumberStatus: 5,
	}
	wait.Succeeded(ctx, t, runcnx, opts, successOpts)
}

// Local Variables:
// compile-command: "go test -tags=e2e -v -run TestBitbucketServerPullRequest$ ."
// End:
