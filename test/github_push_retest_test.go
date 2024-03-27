//go:build e2e
// +build e2e

package test

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/google/go-github/v59/github"
	tgithub "github.com/openshift-pipelines/pipelines-as-code/test/pkg/github"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	twait "github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGithubPushRequestGitOpsCommentOnComment(t *testing.T) {
	opsComment := "/hello-world"
	ctx := context.Background()
	g := &tgithub.PRTest{
		Label:         "Github GitOps push/retest request",
		YamlFiles:     []string{"testdata/pipelinerun-on-comment-annotation.yaml"},
		NoStatusCheck: true,
		TargetRefName: options.MainBranch,
	}
	g.RunPushRequest(ctx, t)
	defer g.TearDown(ctx, t)

	// let's make sure we didn't create any PipelineRuns since we only match on-comment here
	pruns, err := g.Cnx.Clients.Tekton.TektonV1().PipelineRuns(g.TargetNamespace).List(ctx, metav1.ListOptions{})
	assert.NilError(t, err)
	assert.Equal(t, len(pruns.Items), 0)

	g.Cnx.Clients.Log.Infof("Running ops comment %s as Push comment", opsComment)
	_, _, err = g.Provider.Client.Repositories.CreateComment(ctx,
		g.Options.Organization,
		g.Options.Repo, g.SHA,
		&github.RepositoryComment{Body: github.String(opsComment)})
	assert.NilError(t, err)

	waitOpts := twait.Opts{
		RepoName:        g.TargetNamespace,
		Namespace:       g.TargetNamespace,
		MinNumberStatus: len(g.YamlFiles),
		PollTimeout:     twait.DefaultTimeout,
		TargetSHA:       g.SHA,
	}
	g.Cnx.Clients.Log.Info("Waiting for Repository to be updated")
	_, err = twait.UntilRepositoryUpdated(ctx, g.Cnx.Clients, waitOpts)
	assert.NilError(t, err)

	g.Cnx.Clients.Log.Infof("Check if we have the repository set as succeeded")
	repo, err := g.Cnx.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(g.TargetNamespace).Get(ctx, g.TargetNamespace, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, repo.Status[len(repo.Status)-1].Conditions[0].Status, corev1.ConditionTrue)

	pruns, err = g.Cnx.Clients.Tekton.TektonV1().PipelineRuns(g.TargetNamespace).List(ctx, metav1.ListOptions{})
	assert.NilError(t, err)
	assert.Equal(t, len(pruns.Items), len(g.YamlFiles))
	lastPrName := pruns.Items[0].GetName()
	err = twait.RegexpMatchingInPodLog(
		context.Background(),
		g.Cnx,
		g.TargetNamespace,
		fmt.Sprintf("tekton.dev/pipelineRun=%s", lastPrName),
		"step-task",
		*regexp.MustCompile(opsComment),
		2)
	assert.NilError(t, err)
}

func TestGithubPushRequestGitOpsCommentRetest(t *testing.T) {
	ctx := context.Background()
	g := &tgithub.PRTest{
		Label: "Github GitOps push/retest request",
		YamlFiles: []string{
			"testdata/pipelinerun-on-push.yaml", "testdata/pipelinerun.yaml",
		},
	}
	g.RunPushRequest(ctx, t)
	defer g.TearDown(ctx, t)
	comment := "/retest branch:" + g.TargetNamespace

	pruns, err := g.Cnx.Clients.Tekton.TektonV1().PipelineRuns(g.TargetNamespace).List(ctx, metav1.ListOptions{})
	assert.NilError(t, err)
	assert.Equal(t, len(pruns.Items), 2)

	g.Cnx.Clients.Log.Infof("%s on Push Request", comment)
	_, _, err = g.Provider.Client.Repositories.CreateComment(ctx,
		g.Options.Organization,
		g.Options.Repo, g.SHA,
		&github.RepositoryComment{Body: github.String(comment)})
	assert.NilError(t, err)

	waitOpts := twait.Opts{
		RepoName:        g.TargetNamespace,
		Namespace:       g.TargetNamespace,
		MinNumberStatus: 4,
		PollTimeout:     twait.DefaultTimeout,
		TargetSHA:       g.SHA,
	}
	g.Cnx.Clients.Log.Info("Waiting for Repository to be updated")
	_, err = twait.UntilRepositoryUpdated(ctx, g.Cnx.Clients, waitOpts)
	assert.NilError(t, err)

	g.Cnx.Clients.Log.Infof("Check if we have the repository set as succeeded")
	repo, err := g.Cnx.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(g.TargetNamespace).Get(ctx, g.TargetNamespace, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, repo.Status[len(repo.Status)-1].Conditions[0].Status, corev1.ConditionTrue)

	pruns, err = g.Cnx.Clients.Tekton.TektonV1().PipelineRuns(g.TargetNamespace).List(ctx, metav1.ListOptions{})
	assert.NilError(t, err)
	assert.Equal(t, len(pruns.Items), 4)
}

func TestGithubPushRequestGitOpsCommentCancel(t *testing.T) {
	ctx := context.Background()
	g := &tgithub.PRTest{
		Label:            "GitHub Gitops push/cancel request",
		YamlFiles:        []string{"testdata/pipelinerun-on-push.yaml", "testdata/pipelinerun.yaml"},
		SecondController: false,
	}
	g.RunPushRequest(ctx, t)
	defer g.TearDown(ctx, t)

	pruns, err := g.Cnx.Clients.Tekton.TektonV1().PipelineRuns(g.TargetNamespace).List(ctx, metav1.ListOptions{})
	assert.NilError(t, err)
	assert.Equal(t, len(pruns.Items), 2)

	g.Cnx.Clients.Log.Info("/test pipelinerun-on-push on Push Request before canceling")
	_, _, err = g.Provider.Client.Repositories.CreateComment(ctx,
		g.Options.Organization,
		g.Options.Repo, g.SHA,
		&github.RepositoryComment{Body: github.String("/test pipelinerun-on-push branch:" + g.TargetNamespace)})
	assert.NilError(t, err)
	numberOfStatus := 3
	waitOpts := twait.Opts{
		RepoName:        g.TargetNamespace,
		Namespace:       g.TargetNamespace,
		MinNumberStatus: numberOfStatus,
		PollTimeout:     twait.DefaultTimeout,
		TargetSHA:       g.SHA,
	}
	err = twait.UntilPipelineRunCreated(ctx, g.Cnx.Clients, waitOpts)
	assert.NilError(t, err)

	comment := "/cancel pipelinerun-on-push branch:" + g.TargetNamespace
	g.Cnx.Clients.Log.Infof("%s on Push Request", comment)
	_, _, err = g.Provider.Client.Repositories.CreateComment(ctx,
		g.Options.Organization,
		g.Options.Repo, g.SHA,
		&github.RepositoryComment{Body: github.String(comment)})
	assert.NilError(t, err)

	g.Cnx.Clients.Log.Infof("Waiting for Repository to be updated still to %d since it has been canceled", numberOfStatus)
	repo, _ := twait.UntilRepositoryUpdated(ctx, g.Cnx.Clients, waitOpts) // don't check for error, because canceled is not success and this will fail
	cancelled := false
	for _, c := range repo.Status {
		if c.Conditions[0].Reason == "Cancelled" {
			cancelled = true
		}
	}
	assert.Assert(t, cancelled, "No cancelled status in repo Statuses run found")
	assert.Equal(t, repo.Status[len(repo.Status)-1].Conditions[0].Status, corev1.ConditionFalse)

	// make sure the number of items
	pruns, err = g.Cnx.Clients.Tekton.TektonV1().PipelineRuns(g.TargetNamespace).List(ctx, metav1.ListOptions{})
	assert.NilError(t, err)
	assert.Equal(t, len(pruns.Items), numberOfStatus)
	cancelled = false
	for _, pr := range pruns.Items {
		if pr.Status.Conditions[0].Reason == "Cancelled" {
			cancelled = true
		}
	}
	assert.Assert(t, cancelled, "No cancelled pipeline run found")
}
