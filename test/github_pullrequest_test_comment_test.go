//go:build e2e
// +build e2e

package test

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/google/go-github/v61/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/opscomments"
	tgithub "github.com/openshift-pipelines/pipelines-as-code/test/pkg/github"
	twait "github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestGithubPullRequestTest(t *testing.T) {
	if os.Getenv("NIGHTLY_E2E_TEST") != "true" {
		t.Skip("Skipping test since only enabled for nightly")
	}
	ctx := context.TODO()
	g := &tgithub.PRTest{
		Label:            "Github test implicit comment",
		YamlFiles:        []string{"testdata/pipelinerun.yaml", "testdata/pipelinerun-clone.yaml"},
		SecondController: false,
	}
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
	repo, err := twait.UntilRepositoryUpdated(ctx, g.Cnx.Clients, waitOpts)
	assert.NilError(t, err)
	g.Cnx.Clients.Log.Infof("Check if we have the repository set as succeeded")
	assert.Assert(t, repo.Status[len(repo.Status)-1].Conditions[0].Status == corev1.ConditionTrue)
}

func TestGithubSecondOnCommentAnnotation(t *testing.T) {
	g := &tgithub.PRTest{
		Label:            "Github test implicit comment",
		YamlFiles:        []string{"testdata/pipelinerun-on-comment-annotation.yaml"},
		SecondController: true,
		NoStatusCheck:    true,
	}
	ctx := context.Background()
	g.RunPullRequest(ctx, t)
	defer g.TearDown(ctx, t)

	triggerComment := "/hello-world"

	g.Cnx.Clients.Log.Infof("Creating %s custom comment on PullRequest", triggerComment)
	_, _, err := g.Provider.Client.Issues.CreateComment(ctx, g.Options.Organization, g.Options.Repo, g.PRNumber,
		&github.IssueComment{Body: github.String(triggerComment)})
	assert.NilError(t, err)
	sopt := twait.SuccessOpt{
		Title:           fmt.Sprintf("Testing %s with Github APPS integration on %s", g.Label, g.TargetNamespace),
		OnEvent:         opscomments.OnCommentEventType.String(),
		TargetNS:        g.TargetNamespace,
		NumberofPRMatch: 1,
	}
	twait.Succeeded(ctx, t, g.Cnx, g.Options, sopt)

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
	assert.Assert(t, repo.Status[len(repo.Status)-1].Conditions[0].Status == corev1.ConditionTrue)
	lastPrName := repo.Status[len(repo.Status)-1].PipelineRunName

	err = twait.RegexpMatchingInPodLog(context.Background(), g.Cnx, g.TargetNamespace, fmt.Sprintf("tekton.dev/pipelineRun=%s", lastPrName), "step-task", *regexp.MustCompile(triggerComment), "", 2)
	assert.NilError(t, err)
}
