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
	g := tgithub.GitHubTest{
		Label:     "Github PullRequest",
		YamlFiles: []string{"testdata/pipelinerun.yaml"},
	}
	runcnx, ghcnx, opts, targetNS, targetRefName, prNumber, _ := tgithub.RunPullRequest(ctx, t, g)
	defer tgithub.TearDown(ctx, t, runcnx, ghcnx, prNumber, targetRefName, targetNS, opts)
}

func TestGithubPullRequestSecondController(t *testing.T) {
	ctx := context.Background()
	g := tgithub.GitHubTest{
		Label:            "Github PullRequest on Second Controller",
		YamlFiles:        []string{"testdata/pipelinerun.yaml"},
		SecondController: true,
	}
	runcnx, ghcnx, opts, targetNS, targetRefName, prNumber, _ := tgithub.RunPullRequest(ctx, t, g)
	defer tgithub.TearDown(ctx, t, runcnx, ghcnx, prNumber, targetRefName, targetNS, opts)
}

func TestGithubPullRequestMultiples(t *testing.T) {
	if os.Getenv("NIGHTLY_E2E_TEST") != "true" {
		t.Skip("Skipping test since only enabled for nightly")
	}
	ctx := context.Background()
	g := tgithub.GitHubTest{
		Label:            "Github PullRequest multiple",
		YamlFiles:        []string{"testdata/pipelinerun.yaml", "testdata/pipelinerun-clone.yaml"},
		SecondController: true,
	}
	runcnx, ghcnx, opts, targetNS, targetRefName, prNumber, _ := tgithub.RunPullRequest(ctx, t, g)
	defer tgithub.TearDown(ctx, t, runcnx, ghcnx, prNumber, targetRefName, targetNS, opts)
}

func TestGithubPullRequestMatchOnCEL(t *testing.T) {
	if os.Getenv("NIGHTLY_E2E_TEST") != "true" {
		t.Skip("Skipping test since only enabled for nightly")
	}
	g := tgithub.GitHubTest{
		Label:     "Github PullRequest CEL annotations",
		YamlFiles: []string{"testdata/pipelinerun-cel-annotation.yaml"},
	}
	ctx := context.Background()
	runcnx, ghcnx, opts, targetNS, targetRefName, prNumber, _ := tgithub.RunPullRequest(ctx, t, g)
	defer tgithub.TearDown(ctx, t, runcnx, ghcnx, prNumber, targetRefName, targetNS, opts)
}

func TestGithubPullRequestCELMatchOnTitle(t *testing.T) {
	ctx := context.Background()
	g := tgithub.GitHubTest{
		Label:     "Github Pull Request CEL annotations for title match",
		YamlFiles: []string{"testdata/pipelinerun-cel-annotation-for-title-match.yaml"},
	}
	runcnx, ghcnx, opts, targetNS, targetRefName, prNumber, _ := tgithub.RunPullRequest(ctx, t, g)
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
	g := tgithub.GitHubTest{
		Label:     "Github Pull Request on webhook",
		YamlFiles: []string{"testdata/pipelinerun.yaml"},
		Webhook:   true,
	}
	runcnx, ghcnx, opts, targetNS, targetRefName, prNumber, _ := tgithub.RunPullRequest(ctx, t, g)
	defer tgithub.TearDown(ctx, t, runcnx, ghcnx, prNumber, targetRefName, targetNS, opts)
}

// Local Variables:
// compile-command: "go test -tags=e2e -v -info TestGithubPullRequest$ ."
// End:
