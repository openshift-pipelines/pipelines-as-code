//go:build e2e
// +build e2e

package test

import (
	"context"
	"os"
	"testing"

	tgithub "github.com/openshift-pipelines/pipelines-as-code/test/pkg/github"
)

func TestGithubPush(t *testing.T) {
	if os.Getenv("NIGHTLY_E2E_TEST") != "true" {
		t.Skip("Skipping test since only enabled for nightly")
	}
	ctx := context.Background()
	if os.Getenv("TEST_GITHUB_REPO_OWNER_WEBHOOK") == "" {
		runcnx, ghcnx, opts, targetNS, targetRefName, prNumber, _ := tgithub.RunPushRequest(ctx, t,
			"Github Push Request Webhook", []string{"testdata/pipelinerun-on-push.yaml"}, false, true)
		defer tgithub.TearDown(ctx, t, runcnx, ghcnx, prNumber, targetRefName, targetNS, opts)
	} else {
		t.Skip("TEST_GITHUB_REPO_OWNER_WEBHOOK is not set")
	}
	runcnx, ghcnx, opts, targetNS, targetRefName, prNumber, _ := tgithub.RunPushRequest(ctx, t,
		"Github Push Request Apps", []string{"testdata/pipelinerun-on-push.yaml"}, false, false)
	defer tgithub.TearDown(ctx, t, runcnx, ghcnx, prNumber, targetRefName, targetNS, opts)
}

func TestGithubSecondPush(t *testing.T) {
	ctx := context.Background()

	runcnx, ghcnx, opts, targetNS, targetRefName, prNumber, _ := tgithub.RunPushRequest(ctx, t,
		"Github Push Request GHE Apps", []string{"testdata/pipelinerun-on-push.yaml"}, true, false)
	defer tgithub.TearDown(ctx, t, runcnx, ghcnx, prNumber, targetRefName, targetNS, opts)
}

func TestGithubPushRequestCELMatchOnTitle(t *testing.T) {
	ctx := context.Background()
	for _, onWebhook := range []bool{false, true} {
		if onWebhook && os.Getenv("TEST_GITHUB_REPO_OWNER_WEBHOOK") == "" {
			t.Skip("TEST_GITHUB_REPO_OWNER_WEBHOOK is not set")
			continue
		}
		runcnx, ghcnx, opts, targetNS, targetRefName, prNumber, _ := tgithub.RunPushRequest(ctx, t,
			"Github Push Request", []string{"testdata/pipelinerun-cel-annotation-for-title-match.yaml"}, false, onWebhook)
		defer tgithub.TearDown(ctx, t, runcnx, ghcnx, prNumber, targetRefName, targetNS, opts)
	}
}
