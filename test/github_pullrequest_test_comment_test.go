//go:build e2e
// +build e2e

package test

import (
	"context"
	"os"
	"testing"

	"github.com/google/go-github/v56/github"
	tgithub "github.com/openshift-pipelines/pipelines-as-code/test/pkg/github"
	twait "github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGithubPullRequestTest(t *testing.T) {
	if os.Getenv("NIGHTLY_E2E_TEST") != "true" {
		t.Skip("Skipping test since only enabled for nightly")
	}
	ctx := context.TODO()
	runcnx, ghcnx, opts, targetNS, targetRefName, prNumber, sha := tgithub.RunPullRequest(ctx, t, "Github test comment",
		[]string{"testdata/pipelinerun.yaml", "testdata/pipelinerun-clone.yaml"}, false, false)
	defer tgithub.TearDown(ctx, t, runcnx, ghcnx, prNumber, targetRefName, targetNS, opts)

	runcnx.Clients.Log.Infof("Creating /test in PullRequest")
	_, _, err := ghcnx.Client.Issues.CreateComment(ctx,
		opts.Organization,
		opts.Repo, prNumber,
		&github.IssueComment{Body: github.String("/test pipeline")})
	assert.NilError(t, err)

	runcnx.Clients.Log.Infof("Wait for the second repository update to be updated")
	waitOpts := twait.Opts{
		RepoName:        targetNS,
		Namespace:       targetNS,
		MinNumberStatus: 1,
		PollTimeout:     twait.DefaultTimeout,
		TargetSHA:       sha,
	}
	err = twait.UntilRepositoryUpdated(ctx, runcnx.Clients, waitOpts)
	assert.NilError(t, err)

	runcnx.Clients.Log.Infof("Check if we have the repository set as succeeded")
	repo, err := runcnx.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(targetNS).Get(ctx, targetNS, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Assert(t, repo.Status[len(repo.Status)-1].Conditions[0].Status == corev1.ConditionTrue)
}
