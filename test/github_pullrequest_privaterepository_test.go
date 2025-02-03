//go:build e2e
// +build e2e

package test

import (
	"context"
	"os"
	"regexp"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/cctx"
	tgithub "github.com/openshift-pipelines/pipelines-as-code/test/pkg/github"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	"gotest.tools/v3/assert"
)

func TestGithubPullRequestGitClone(t *testing.T) {
	if os.Getenv("NIGHTLY_E2E_TEST") != "true" {
		t.Skip("Skipping test since only enabled for nightly")
	}
	ctx := context.Background()
	g := &tgithub.PRTest{
		Label:     "Github - Private Repo",
		YamlFiles: []string{"testdata/pipelinerun_git_clone_private.yaml"},
	}
	g.RunPullRequest(ctx, t)

	ctx, err := cctx.GetControllerCtxInfo(ctx, g.Cnx)
	assert.NilError(t, err)

	maxLines := int64(20)
	assert.NilError(t, wait.RegexpMatchingInControllerLog(ctx, g.Cnx, *regexp.MustCompile(".*fetched git-clone task"),
		10, "controller", &maxLines), "Error while checking the logs of the pipelines-as-code controller pod")
	defer g.TearDown(ctx, t)
}

func TestGithubSecondPullRequestGitClone(t *testing.T) {
	ctx := context.Background()
	g := &tgithub.PRTest{
		Label:            "Github GHE - Private Repo",
		YamlFiles:        []string{"testdata/pipelinerun_git_clone_private.yaml"},
		SecondController: true,
	}
	g.RunPullRequest(ctx, t)
	defer g.TearDown(ctx, t)
}

func TestGithubPullRequestPrivateRepositoryOnWebhook(t *testing.T) {
	if os.Getenv("NIGHTLY_E2E_TEST") != "true" {
		t.Skip("Skipping test since only enabled for nightly")
	}
	ctx := context.Background()
	g := &tgithub.PRTest{
		Label:     "Github Rerequest",
		YamlFiles: []string{"testdata/pipelinerun_git_clone_private.yaml"},
		Webhook:   true,
	}
	g.RunPullRequest(ctx, t)
	defer g.TearDown(ctx, t)
}

// Local Variables:
// compile-command: "go test -tags=e2e -v -info TestPullRequestPrivateRepository$ ."
// End:
