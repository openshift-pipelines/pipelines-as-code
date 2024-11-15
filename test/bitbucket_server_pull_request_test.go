//go:build e2e
// +build e2e

package test

import (
	"context"
	"fmt"
	"os"
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
	bitbucketWSOwner := os.Getenv("TEST_BITBUCKET_SERVER_E2E_REPOSITORY")

	ctx, runcnx, opts, client, err := tbbs.Setup(ctx)
	assert.NilError(t, err)

	repo := tbbs.CreateCRD(ctx, t, client, runcnx, bitbucketWSOwner, targetNS)
	runcnx.Clients.Log.Infof("Repository %s has been created", repo.Name)
	defer tbbs.TearDownNs(ctx, t, runcnx, targetNS)

	title := "TestPullRequest - " + targetNS
	numberOfFiles := 5
	files := map[string]string{}
	for i := range numberOfFiles {
		files[fmt.Sprintf("pipelinerun-%d.yaml", i)] = "testdata/pipelinerun.yaml"
	}

	pr := tbbs.CreatePR(ctx, t, client, runcnx, bitbucketWSOwner, files, title, targetNS)
	runcnx.Clients.Log.Infof("Pull Request with title '%s' is created", pr.Title)
	defer tbbs.TearDownBranch(ctx, t, runcnx, client, pr.Number, bitbucketWSOwner, targetNS)

	successOpts := wait.SuccessOpt{
		TargetNS:        targetNS,
		OnEvent:         triggertype.PullRequest.String(),
		NumberofPRMatch: 5,
		MinNumberStatus: 5,
	}
	wait.Succeeded(ctx, t, runcnx, opts, successOpts)
}
