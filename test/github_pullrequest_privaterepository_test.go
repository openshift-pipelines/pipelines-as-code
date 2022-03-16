//go:build e2e
// +build e2e

package test

import (
	"context"
	"testing"

	tgithub "github.com/openshift-pipelines/pipelines-as-code/test/pkg/github"
)

func TestGithubPullRequestPrivateRepository(t *testing.T) {
	t.Skip()
	ctx := context.TODO()
	runcnx, ghcnx, opts, targetNS, targetRefName, prNumber, _ := tgithub.RunPullRequest(ctx, t, "Github Private Repo", "testdata/pipelinerun_git_clone_private.yaml", false)
	defer tgithub.TearDown(ctx, t, runcnx, ghcnx, prNumber, targetRefName, targetNS, opts)
}

func TestGithubPullRequestPrivateRepositoryOnWebhook(t *testing.T) {
	t.Skip()
	ctx := context.TODO()
	runcnx, ghcnx, opts, targetNS, targetRefName, prNumber, _ := tgithub.RunPullRequest(ctx, t, "Github Private Repo OnWebhook", "testdata/pipelinerun_git_clone_private.yaml", true)
	defer tgithub.TearDown(ctx, t, runcnx, ghcnx, prNumber, targetRefName, targetNS, opts)
}

// Local Variables:
// compile-command: "go test -tags=e2e -v -info TestPullRequestPrivateRepository$ ."
// End:
