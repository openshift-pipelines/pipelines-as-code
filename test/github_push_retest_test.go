//go:build e2e
// +build e2e

package test

import (
	"context"
	"regexp"
	"testing"

	"github.com/google/go-github/v64/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/cctx"
	tgithub "github.com/openshift-pipelines/pipelines-as-code/test/pkg/github"
	twait "github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGithubPushRequestGitOpsCommentRetest(t *testing.T) {
	ctx := context.Background()
	g := &tgithub.PRTest{
		Label:     "Github GitOps push/retest request",
		YamlFiles: []string{"testdata/pipelinerun-on-push.yaml", "testdata/pipelinerun.yaml"},
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

	for i := range pruns.Items {
		sData, err := g.Cnx.Clients.Kube.CoreV1().Secrets(g.TargetNamespace).Get(ctx, pruns.Items[i].GetAnnotations()[keys.GitAuthSecret], metav1.GetOptions{})
		assert.NilError(t, err)
		assert.Assert(t, string(sData.Data["git-provider-token"]) != "")
		assert.Assert(t, string(sData.Data[".git-credentials"]) != "")
		assert.Assert(t, string(sData.Data[".gitconfig"]) != "")
	}
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

	ctx, err := cctx.GetControllerCtxInfo(ctx, g.Cnx)
	assert.NilError(t, err)

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
		if c.Conditions[0].Reason == tektonv1.TaskRunReasonCancelled.String() {
			cancelled = true
		}
	}

	// this went too fast so at least we check it was requested for it
	if !cancelled {
		reg := regexp.MustCompile(".*pipelinerun.*is done.*skipping cancellation.*")
		err = twait.RegexpMatchingInControllerLog(ctx, g.Cnx, *reg, 10, "controller", github.Int64(20))
		if err != nil {
			t.Errorf("neither a cancelled pipelinerun in repo status or a request to skip the cancellation in the controller log was found: %s", err.Error())
		}
		return
	}

	// make sure the number of items
	pruns, err = g.Cnx.Clients.Tekton.TektonV1().PipelineRuns(g.TargetNamespace).List(ctx, metav1.ListOptions{})
	assert.NilError(t, err)
	assert.Equal(t, len(pruns.Items), numberOfStatus)
	cancelled = false
	for _, pr := range pruns.Items {
		if pr.Status.Conditions[0].Reason == tektonv1.TaskRunReasonCancelled.String() {
			cancelled = true
		}
	}
	assert.Assert(t, cancelled, "No cancelled pipeline run found")
}
