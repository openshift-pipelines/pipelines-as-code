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
	t.Parallel()
	ctx := context.Background()
	runcnx, ghcnx, opts, targetNS, targetRefName, prNumber, _ := tgithub.RunPullRequest(ctx, t, "Github PullRequest", "testdata/pipelinerun.yaml", false)
	defer tgithub.TearDown(ctx, t, runcnx, ghcnx, prNumber, targetRefName, targetNS, opts)
}

func TestGithubPullRequestMatchOnCEL(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	runcnx, ghcnx, opts, targetNS, targetRefName, prNumber, _ := tgithub.RunPullRequest(ctx, t, "Github PullRequest",
		"testdata/pipelinerun-cel-annotation.yaml", false)
	defer tgithub.TearDown(ctx, t, runcnx, ghcnx, prNumber, targetRefName, targetNS, opts)
}

func TestGithubPullRequestWebhook(t *testing.T) {
	if os.Getenv("TEST_GITHUB_REPO_OWNER_WEBHOOK") == "" {
		t.Skip("TEST_GITHUB_REPO_OWNER_WEBHOOK is not set")
		return
	}
	ctx := context.Background()

	runcnx, ghcnx, opts, targetNS, targetRefName, prNumber, _ := tgithub.RunPullRequest(ctx, t, "Github PullRequest onWebhook", "testdata/pipelinerun.yaml", true)
	defer tgithub.TearDown(ctx, t, runcnx, ghcnx, prNumber, targetRefName, targetNS, opts)
}

// Local Variables:
// compile-command: "go test -tags=e2e -v -info TestGithubPullRequest$ ."
// End:
