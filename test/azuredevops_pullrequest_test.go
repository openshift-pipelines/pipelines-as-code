//go:build e2e
// +build e2e

package test

import (
	"context"
	"testing"

	tazdevops "github.com/openshift-pipelines/pipelines-as-code/test/pkg/azuredevops"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	"github.com/tektoncd/pipeline/pkg/names"
)

func TestAzureDevopsPullRequest(t *testing.T) {
	targetNS := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-ns")
	ctx := context.Background()

	runcnx, opts, azprovider, err := tazdevops.Setup(ctx)
	if err != nil {
		t.Skip(err.Error())
		return
	}
	tazdevops.CreateCRD(ctx, t, azprovider, runcnx, opts, targetNS)
	targetRefName := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-test")
	title := "TestPullRequest - " + targetRefName

	PullRequestId, RefName, PushID := tazdevops.MakePR(ctx, t, azprovider, opts, title, targetNS, targetRefName)
	defer tazdevops.TearDown(ctx, t, runcnx, azprovider, opts, PullRequestId, targetNS, RefName, PushID)

	sopt := wait.SuccessOpt{
		TargetNS:        targetNS,
		OnEvent:         "git.pullrequest.created",
		NumberofPRMatch: 1,
		SHA:             *PushID,
		Title:           title,
		MinNumberStatus: 1,
	}
	wait.Succeeded(ctx, t, runcnx, opts, sopt)
}
