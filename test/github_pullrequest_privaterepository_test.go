//go:build e2e
// +build e2e

package test

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/cctx"
	tgithub "github.com/openshift-pipelines/pipelines-as-code/test/pkg/github"
	twait "github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestGithubPullRequestPrivateRepositoryRelativeTask(t *testing.T) {
	ctx := context.Background()
	g := &tgithub.PRTest{
		Label:     "Github - Private Repo Relative Task",
		YamlFiles: []string{"testdata/pipelinerun_remote_pipeline_annotations.yaml"},
		ExtraArgs: map[string]string{
			"RemotePipeline": "https://github.com/chmouel/scratchmyback/blob/remote-pipeline/pipeline.yaml",
		},
	}
	g.RunPullRequest(ctx, t)

	waitOpts := twait.Opts{
		RepoName:        g.TargetNamespace,
		Namespace:       g.TargetNamespace,
		MinNumberStatus: 1,
		PollTimeout:     twait.DefaultTimeout,
		TargetSHA:       g.SHA,
	}
	repo, err := twait.UntilRepositoryUpdated(ctx, g.Cnx.Clients, waitOpts)
	assert.NilError(t, err)
	g.Cnx.Clients.Log.Infof("Check if we have the repository set as succeeded")
	assert.Equal(t, repo.Status[len(repo.Status)-1].Conditions[0].Status, corev1.ConditionTrue)
	lastPrName := repo.Status[len(repo.Status)-1].PipelineRunName

	err = twait.RegexpMatchingInPodLog(context.Background(),
		g.Cnx,
		g.TargetNamespace,
		fmt.Sprintf("tekton.dev/pipelineRun=%s", lastPrName),
		"step-echo",
		*regexp.MustCompile("Hello from Relative Task!"), "", 2)
	assert.NilError(t, err)
}

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
	assert.NilError(t, twait.RegexpMatchingInControllerLog(ctx, g.Cnx, *regexp.MustCompile(".*fetched git-clone task"),
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
