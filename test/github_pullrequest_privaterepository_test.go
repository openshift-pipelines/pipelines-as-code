//go:build e2e
// +build e2e

package test

import (
	"context"
	"os"
	"testing"

	tgithub "github.com/openshift-pipelines/pipelines-as-code/test/pkg/github"
)

func TestGithubPullRequestGitClone(t *testing.T) {
	if os.Getenv("NIGHTLY_E2E_TEST") != "true" {
		t.Skip("Skipping test since only enabled for nightly")
	}
	ctx := context.Background()
	runcnx, ghcnx, opts, targetNS, targetRefName, prNumber, _ := tgithub.RunPullRequest(ctx, t,
		"Github Private Repo", []string{"testdata/pipelinerun_git_clone_private.yaml"}, false, false)
	defer tgithub.TearDown(ctx, t, runcnx, ghcnx, prNumber, targetRefName, targetNS, opts)
}

func TestGithubSecondPullRequestGitClone(t *testing.T) {
	ctx := context.Background()
	runcnx, ghcnx, opts, targetNS, targetRefName, prNumber, _ := tgithub.RunPullRequest(ctx, t,
		"Github Private Repo", []string{"testdata/pipelinerun_git_clone_private.yaml"}, true, false)
	defer tgithub.TearDown(ctx, t, runcnx, ghcnx, prNumber, targetRefName, targetNS, opts)
}

func TestGithubPullRequestPrivateRepositoryOnWebhook(t *testing.T) {
	if os.Getenv("NIGHTLY_E2E_TEST") != "true" {
		t.Skip("Skipping test since only enabled for nightly")
	}
	ctx := context.Background()
	runcnx, ghcnx, opts, targetNS, targetRefName, prNumber, _ := tgithub.RunPullRequest(ctx, t,
		"Github Private Repo OnWebhook", []string{"testdata/pipelinerun_git_clone_private.yaml"}, false, true)
	defer tgithub.TearDown(ctx, t, runcnx, ghcnx, prNumber, targetRefName, targetNS, opts)
}

// Local Variables:
// compile-command: "go test -tags=e2e -v -info TestPullRequestPrivateRepository$ ."
// End:
