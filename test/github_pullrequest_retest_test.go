//go:build e2e
// +build e2e

package test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/go-github/v71/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	tgithub "github.com/openshift-pipelines/pipelines-as-code/test/pkg/github"
	twait "github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestGithubSecondPullRequestGitopsCommentRetest will test the retest
// functionality of a GitHub pull request.
func TestGithubSecondPullRequestGitopsCommentRetest(t *testing.T) {
	if os.Getenv("NIGHTLY_E2E_TEST") != "true" {
		t.Skip("Skipping test since only enabled for nightly")
	}
	ctx := context.Background()
	g := &tgithub.PRTest{
		Label:            "Github retest comment",
		YamlFiles:        []string{"testdata/pipelinerun.yaml"},
		SecondController: true,
		NoStatusCheck:    true,
	}
	g.RunPullRequest(ctx, t)
	defer g.TearDown(ctx, t)

	g.Cnx.Clients.Log.Infof("Creating /retest in PullRequest")
	_, _, err := g.Provider.Client().Issues.CreateComment(ctx,
		g.Options.Organization,
		g.Options.Repo, g.PRNumber,
		&github.IssueComment{Body: github.Ptr("/retest")})
	assert.NilError(t, err)

	g.Cnx.Clients.Log.Infof("Wait for the second repository update to be updated")
	waitOpts := twait.Opts{
		RepoName:        g.TargetNamespace,
		Namespace:       g.TargetNamespace,
		MinNumberStatus: 1,
		PollTimeout:     twait.DefaultTimeout,
		TargetSHA:       g.SHA,
	}
	_, err = twait.UntilRepositoryUpdated(ctx, g.Cnx.Clients, waitOpts)
	assert.NilError(t, err)

	g.Cnx.Clients.Log.Infof("Check if we have the repository set as succeeded")
	repo, err := g.Cnx.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(g.TargetNamespace).Get(ctx, g.TargetNamespace, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, repo.Status[len(repo.Status)-1].Conditions[0].Status, corev1.ConditionTrue)
}

// TestGithubSecondPullRequestRetest tests the retest functionality of a GitHub pull request.
// It sets up a pull request, triggers a retest comment, waits for the repository to be updated,
// and verifies that the repository status is set to succeeded and the correct number of PipelineRuns are created.
func TestGithubSecondPullRequestGitopsCommentCancel(t *testing.T) {
	ctx := context.Background()
	g := &tgithub.PRTest{
		Label:            "Github PullRequest Cancel",
		YamlFiles:        []string{"testdata/pipelinerun.yaml", "testdata/pipelinerun-gitops.yaml"},
		SecondController: true,
	}
	g.RunPullRequest(ctx, t)
	defer g.TearDown(ctx, t)

	pruns, err := g.Cnx.Clients.Tekton.TektonV1().PipelineRuns(g.TargetNamespace).List(ctx, metav1.ListOptions{})
	assert.NilError(t, err)
	assert.Equal(t, len(pruns.Items), 2)

	g.Cnx.Clients.Log.Info("/test pr-gitops-comment on Pull Request before canceling")
	_, _, err = g.Provider.Client().Issues.CreateComment(ctx,
		g.Options.Organization,
		g.Options.Repo,
		g.PRNumber,
		&github.IssueComment{Body: github.Ptr("/test pr-gitops-comment")},
	)
	waitOpts := twait.Opts{
		RepoName:        g.TargetNamespace,
		Namespace:       g.TargetNamespace,
		MinNumberStatus: 3,
		PollTimeout:     twait.DefaultTimeout,
		TargetSHA:       g.SHA,
	}
	assert.NilError(t, err)
	err = twait.UntilPipelineRunCreated(ctx, g.Cnx.Clients, waitOpts)
	assert.NilError(t, err)

	g.Cnx.Clients.Log.Infof("/cancel pr-gitops-comment on Pull Request")
	_, _, err = g.Provider.Client().Issues.CreateComment(ctx,
		g.Options.Organization,
		g.Options.Repo, g.PRNumber,
		&github.IssueComment{Body: github.Ptr("/cancel pr-gitops-comment")})
	assert.NilError(t, err)

	waitOpts = twait.Opts{
		RepoName:        g.TargetNamespace,
		Namespace:       g.TargetNamespace,
		MinNumberStatus: 3,
		PollTimeout:     twait.DefaultTimeout,
		TargetSHA:       g.SHA,
	}
	g.Cnx.Clients.Log.Info("Waiting for Repository to be updated")
	_, err = twait.UntilRepositoryUpdated(ctx, g.Cnx.Clients, waitOpts)
	assert.NilError(t, err)

	g.Cnx.Clients.Log.Infof("Check if we have the repository set as succeeded")
	repo, err := g.Cnx.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(g.TargetNamespace).Get(ctx, g.TargetNamespace, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, repo.Status[len(repo.Status)-1].Conditions[0].Status, corev1.ConditionFalse)

	pruns, err = g.Cnx.Clients.Tekton.TektonV1().PipelineRuns(g.TargetNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", keys.SHA, g.SHA),
	})
	assert.NilError(t, err)
	assert.Equal(t, len(pruns.Items), 3)

	// go over all pruns check that at least one is canceled and the other two are succeeded
	canceledCount := 0
	succeededCount := 0
	unknownCount := 0
	for _, prun := range pruns.Items {
		for _, condition := range prun.Status.Conditions {
			if condition.Type == "Succeeded" {
				switch condition.Status {
				case corev1.ConditionFalse:
					canceledCount++
				case corev1.ConditionTrue:
					succeededCount++
				case corev1.ConditionUnknown:
					unknownCount++
				}
			}
		}
	}
	assert.Equal(t, canceledCount, 1, "should have one canceled PipelineRun")
	assert.Equal(t, succeededCount, 2, "should have two succeeded PipelineRuns")
	assert.Equal(t, unknownCount, 0, "should have zero unknown PipelineRuns: %+v", pruns.Items)
}
