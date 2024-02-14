//go:build e2e
// +build e2e

package test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-github/v59/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/opscomments"
	tgithub "github.com/openshift-pipelines/pipelines-as-code/test/pkg/github"
	twait "github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func testGitOpsTestComment(ctx context.Context, t *testing.T, g *tgithub.PRTest) {
	g.RunPullRequest(ctx, t)
	defer g.TearDown(ctx, t)

	g.Cnx.Clients.Log.Infof("Creating /test in PullRequest")
	_, _, err := g.Provider.Client.Issues.CreateComment(ctx,
		g.Options.Organization,
		g.Options.Repo, g.PRNumber,
		&github.IssueComment{Body: github.String("/test pipeline")})
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
	assert.Assert(t, repo.Status[len(repo.Status)-1].Conditions[0].Status == corev1.ConditionTrue)
}

func testGitOpsCommentOnAnnotation(ctx context.Context, t *testing.T, g *tgithub.PRTest) {
	g.RunPullRequest(ctx, t)
	defer g.TearDown(ctx, t)

	g.Cnx.Clients.Log.Infof("Creating /hello-world custom comment on PullRequest")
	_, _, err := g.Provider.Client.Issues.CreateComment(ctx, g.Options.Organization, g.Options.Repo, g.PRNumber,
		&github.IssueComment{Body: github.String("/hello-world")})
	assert.NilError(t, err)
	sopt := twait.SuccessOpt{
		Title:           fmt.Sprintf("Testing %s with Github APPS integration on %s", g.Label, g.TargetNamespace),
		OnEvent:         opscomments.OnCommentEventType.String(),
		TargetNS:        g.TargetNamespace,
		NumberofPRMatch: 1,
	}
	twait.Succeeded(ctx, t, g.Cnx, g.Options, sopt)
}

func testGitOpsRetestCancel(ctx context.Context, t *testing.T, g *tgithub.PRTest) {
	g.RunPullRequest(ctx, t)
	defer g.TearDown(ctx, t)

	g.Cnx.Clients.Log.Infof("Creating /retest in PullRequest")
	_, _, err := g.Provider.Client.Issues.CreateComment(ctx,
		g.Options.Organization,
		g.Options.Repo, g.PRNumber,
		&github.IssueComment{Body: github.String("/retest")})
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

// TestGithubPullRequestGitOpsComments tests GitOps comments /test, /retest and /cancel commands.
func TestGithubPullRequestGitOpsComments(t *testing.T) {
	ctx := context.Background()
	g := &tgithub.PRTest{
		Label:     "Github PullRequest GitOps Comments test",
		YamlFiles: []string{"testdata/pipelinerun.yaml", "testdata/pipelinerun-gitops.yaml"},
	}
	g.RunPullRequest(ctx, t)
	defer g.TearDown(ctx, t)

	pruns, err := g.Cnx.Clients.Tekton.TektonV1().PipelineRuns(g.TargetNamespace).List(ctx, metav1.ListOptions{})
	assert.NilError(t, err)
	assert.Equal(t, len(pruns.Items), 2)

	tests := []struct {
		name, comment string
		prNum         int
	}{
		{
			name:    "Retest",
			comment: "/retest pr-gitops-comment",
			prNum:   3,
		},
		{
			name:    "Test and Cancel PipelineRun",
			comment: "/cancel pr-gitops-comment",
			prNum:   4,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			waitOpts := twait.Opts{
				RepoName:        g.TargetNamespace,
				Namespace:       g.TargetNamespace,
				MinNumberStatus: tt.prNum,
				PollTimeout:     twait.DefaultTimeout,
				TargetSHA:       g.SHA,
			}
			if tt.comment == "/cancel pr-gitops-comment" {
				g.Cnx.Clients.Log.Info("/test pr-gitops-comment on Pull Request before canceling")
				_, _, err := g.Provider.Client.Issues.CreateComment(ctx,
					g.Options.Organization,
					g.Options.Repo, g.PRNumber,
					&github.IssueComment{Body: github.String("/test pr-gitops-comment")})
				assert.NilError(t, err)
				err = twait.UntilPipelineRunCreated(ctx, g.Cnx.Clients, waitOpts)
				assert.NilError(t, err)
			}
			g.Cnx.Clients.Log.Infof("%s on Pull Request", tt.comment)
			_, _, err = g.Provider.Client.Issues.CreateComment(ctx,
				g.Options.Organization,
				g.Options.Repo, g.PRNumber,
				&github.IssueComment{Body: github.String(tt.comment)})
			assert.NilError(t, err)

			g.Cnx.Clients.Log.Info("Waiting for Repository to be updated")
			_, err = twait.UntilRepositoryUpdated(ctx, g.Cnx.Clients, waitOpts)
			assert.NilError(t, err)

			g.Cnx.Clients.Log.Infof("Check if we have the repository set as succeeded")
			repo, err := g.Cnx.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(g.TargetNamespace).Get(ctx, g.TargetNamespace, metav1.GetOptions{})
			assert.NilError(t, err)
			if tt.comment == "/cancel pr-gitops-comment" {
				assert.Equal(t, repo.Status[len(repo.Status)-1].Conditions[0].Status, corev1.ConditionFalse)
			} else {
				assert.Equal(t, repo.Status[len(repo.Status)-1].Conditions[0].Status, corev1.ConditionTrue)
			}

			pruns, err = g.Cnx.Clients.Tekton.TektonV1().PipelineRuns(g.TargetNamespace).List(ctx, metav1.ListOptions{
				LabelSelector: fmt.Sprintf("%s=%s", keys.SHA, g.SHA),
			})
			assert.NilError(t, err)
			assert.Equal(t, len(pruns.Items), tt.prNum)
		})
	}
}

func TestGithubSecondPullRequestTest(t *testing.T) {
	ctx := context.TODO()
	g := &tgithub.PRTest{
		Label:            "Github test implicit comment",
		YamlFiles:        []string{"testdata/pipelinerun.yaml", "testdata/pipelinerun-clone.yaml"},
		SecondController: true,
	}
	testGitOpsTestComment(ctx, t, g)
}

func TestGithubWebhookPullRequestTest(t *testing.T) {
	ctx := context.TODO()
	g := &tgithub.PRTest{
		Label:     "Github test implicit comment",
		YamlFiles: []string{"testdata/pipelinerun.yaml", "testdata/pipelinerun-clone.yaml"},
		Webhook:   true,
	}
	testGitOpsTestComment(ctx, t, g)
}

func TestGithubSecondOnCommentAnnotation(t *testing.T) {
	g := &tgithub.PRTest{
		Label:            "Github test implicit comment on second controller",
		YamlFiles:        []string{"testdata/pipelinerun-on-comment-annotation.yaml"},
		SecondController: true,
		NoStatusCheck:    true,
	}
	ctx := context.Background()
	testGitOpsCommentOnAnnotation(ctx, t, g)
}

func TestGithubWebhookOnCommentAnnotation(t *testing.T) {
	g := &tgithub.PRTest{
		Label:         "Github test implicit comment on webhook",
		YamlFiles:     []string{"testdata/pipelinerun-on-comment-annotation.yaml"},
		Webhook:       true,
		NoStatusCheck: true,
	}
	ctx := context.Background()
	testGitOpsCommentOnAnnotation(ctx, t, g)
}

func TestGithubSecondGitopsCancelRetest(t *testing.T) {
	ctx := context.Background()
	g := &tgithub.PRTest{
		Label:            "Github retest comment on Second Controller",
		YamlFiles:        []string{"testdata/pipelinerun.yaml"},
		SecondController: true,
		NoStatusCheck:    true,
	}
	testGitOpsRetestCancel(ctx, t, g)
}

func TestGithubWebhookGitopsCancelRetest(t *testing.T) {
	ctx := context.Background()
	g := &tgithub.PRTest{
		Label:         "Github retest comment on webhook",
		YamlFiles:     []string{"testdata/pipelinerun.yaml"},
		Webhook:       true,
		NoStatusCheck: true,
	}
	testGitOpsRetestCancel(ctx, t, g)
}
