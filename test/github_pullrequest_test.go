//go:build e2e
// +build e2e

package test

import (
	"context"
	"os"
	"testing"

	tgithub "github.com/openshift-pipelines/pipelines-as-code/test/pkg/github"
)

func TestGithubPullRequest(t *testing.T) {
	ctx := context.Background()
	runcnx, ghcnx, opts, targetNS, targetRefName, prNumber, _ := tgithub.RunPullRequest(ctx, t, "Github PullRequest",
		[]string{"testdata/pipelinerun.yaml"}, false, false)
	defer tgithub.TearDown(ctx, t, runcnx, ghcnx, prNumber, targetRefName, targetNS, opts)
}

func TestGithubPullRequestSecondController(t *testing.T) {
	ctx := context.Background()
	runcnx, ghcnx, opts, targetNS, targetRefName, prNumber, _ := tgithub.RunPullRequest(
		ctx, t, "Github PullRequest", []string{"testdata/pipelinerun.yaml"}, true, false,
	)
	defer tgithub.TearDown(ctx, t, runcnx, ghcnx, prNumber, targetRefName, targetNS, opts)
}

func TestGithubPullRequestMultiples(t *testing.T) {
	if os.Getenv("NIGHTLY_E2E_TEST") != "true" {
		t.Skip("Skipping test since only enabled for nightly")
	}
	ctx := context.Background()
	runcnx, ghcnx, opts, targetNS, targetRefName, prNumber, _ := tgithub.RunPullRequest(ctx, t, "Github PullRequest",
		[]string{"testdata/pipelinerun.yaml", "testdata/pipelinerun-clone.yaml"}, false, false)
	defer tgithub.TearDown(ctx, t, runcnx, ghcnx, prNumber, targetRefName, targetNS, opts)
}

func TestGithubPullRequestMatchOnCEL(t *testing.T) {
	if os.Getenv("NIGHTLY_E2E_TEST") != "true" {
		t.Skip("Skipping test since only enabled for nightly")
	}
	ctx := context.Background()
	runcnx, ghcnx, opts, targetNS, targetRefName, prNumber, _ := tgithub.RunPullRequest(ctx, t, "Github PullRequest",
		[]string{"testdata/pipelinerun-cel-annotation.yaml"}, false, false)
	defer tgithub.TearDown(ctx, t, runcnx, ghcnx, prNumber, targetRefName, targetNS, opts)
}

func TestGithubPullRequestCELMatchOnTitle(t *testing.T) {
	ctx := context.Background()
	runcnx, ghcnx, opts, targetNS, targetRefName, prNumber, _ := tgithub.RunPullRequest(ctx, t, "Github PullRequest",
		[]string{"testdata/pipelinerun-cel-annotation-for-title-match.yaml"}, false, false)
	defer tgithub.TearDown(ctx, t, runcnx, ghcnx, prNumber, targetRefName, targetNS, opts)
}

func TestGithubPullRequestWebhook(t *testing.T) {
	if os.Getenv("NIGHTLY_E2E_TEST") != "true" {
		t.Skip("Skipping test since only enabled for nightly")
	}
	if os.Getenv("TEST_GITHUB_REPO_OWNER_WEBHOOK") == "" {
		t.Skip("TEST_GITHUB_REPO_OWNER_WEBHOOK is not set")
		return
	}
	ctx := context.Background()

	runcnx, ghcnx, opts, targetNS, targetRefName, prNumber, _ := tgithub.RunPullRequest(ctx, t,
		"Github PullRequest onWebhook", []string{"testdata/pipelinerun.yaml"}, false, true)
	defer tgithub.TearDown(ctx, t, runcnx, ghcnx, prNumber, targetRefName, targetNS, opts)
}

// Local Variables:
// compile-command: "go test -tags=e2e -v -info TestGithubPullRequest$ ."
// End:
