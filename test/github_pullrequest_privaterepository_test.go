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

	g := tgithub.GitHubTest{
		Label:     "Github Private Repo",
		YamlFiles: []string{"testdata/pipelinerun_git_clone_private.yaml"},
	}
	runcnx, ghcnx, opts, targetNS, targetRefName, prNumber, _ := tgithub.RunPullRequest(ctx, t, g)
	defer tgithub.TearDown(ctx, t, runcnx, ghcnx, prNumber, targetRefName, targetNS, opts)
}

func TestGithubSecondPullRequestGitClone(t *testing.T) {
	ctx := context.Background()
	g := tgithub.GitHubTest{
		Label:            "Github Private Repo on Second controller",
		YamlFiles:        []string{"testdata/pipelinerun_git_clone_private.yaml"},
		SecondController: true,
	}
	runcnx, ghcnx, opts, targetNS, targetRefName, prNumber, _ := tgithub.RunPullRequest(ctx, t, g)
	defer tgithub.TearDown(ctx, t, runcnx, ghcnx, prNumber, targetRefName, targetNS, opts)
}

func TestGithubPullRequestPrivateRepositoryOnWebhook(t *testing.T) {
	if os.Getenv("NIGHTLY_E2E_TEST") != "true" {
		t.Skip("Skipping test since only enabled for nightly")
	}
	ctx := context.Background()
	g := tgithub.GitHubTest{
		Label:     "Github Private Repo on webhook",
		YamlFiles: []string{"testdata/pipelinerun_git_clone_private.yaml"},
		Webhook:   true,
	}

	runcnx, ghcnx, opts, targetNS, targetRefName, prNumber, _ := tgithub.RunPullRequest(ctx, t, g)
	defer tgithub.TearDown(ctx, t, runcnx, ghcnx, prNumber, targetRefName, targetNS, opts)
}

// Local Variables:
// compile-command: "go test -tags=e2e -v -info TestPullRequestPrivateRepository$ ."
// End:
