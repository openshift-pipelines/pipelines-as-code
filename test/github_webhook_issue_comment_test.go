//go:build e2e

package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-github/v81/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
	tgithub "github.com/openshift-pipelines/pipelines-as-code/test/pkg/github"
	twait "github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"gotest.tools/v3/assert"
)

func setupTestWebhookPipeline(t *testing.T, comment string) {
	t.Helper()

	ctx := context.Background()
	g := &tgithub.PRTest{
		Label:     fmt.Sprintf("Github webhook %s comment", comment),
		YamlFiles: []string{"testdata/pipelinerun.yaml"},
		Webhook:   true,
	}
	g.RunPullRequest(ctx, t)
	defer g.TearDown(ctx, t)

	// Wait for initial PR to be processed
	time.Sleep(5 * time.Second)

	_, _, err := g.Provider.Client().Issues.CreateComment(ctx,
		g.Options.Organization,
		g.Options.Repo,
		g.PRNumber,
		&github.IssueComment{Body: github.Ptr(comment)})
	assert.NilError(t, err)

	// Verify pipeline runs are created
	sopt := twait.SuccessOpt{
		Title:           g.CommitTitle,
		OnEvent:         triggertype.PullRequest.String(),
		TargetNS:        g.TargetNamespace,
		NumberofPRMatch: 1,
		SHA:             g.SHA,
	}
	twait.Succeeded(ctx, t, g.Cnx, g.Options, sopt)
}

// TestGithubWebhookIssueCommentRetest tests /retest GitOps command via webhook.
func TestGithubWebhookIssueCommentRetest(t *testing.T) {
	setupTestWebhookPipeline(t, "/retest")
}

// TestGithubWebhookIssueCommentTestSpecific tests /test <pipeline> GitOps command via webhook.
func TestGithubWebhookIssueCommentTestSpecific(t *testing.T) {
	setupTestWebhookPipeline(t, "/test pipelinerun")
}

// TestGithubWebhookIssueCommentCancel tests /cancel GitOps command via webhook.
func TestGithubWebhookIssueCommentCancel(t *testing.T) {
	ctx := context.Background()
	g := &tgithub.PRTest{
		Label:         "Github webhook /cancel",
		YamlFiles:     []string{"testdata/pipelinerun-gitops.yaml"},
		Webhook:       true,
		NoStatusCheck: true,
	}
	g.RunPullRequest(ctx, t)
	defer g.TearDown(ctx, t)

	// Wait for the PipelineRun to be created
	waitOpts := twait.Opts{
		RepoName:        g.TargetNamespace,
		Namespace:       g.TargetNamespace,
		MinNumberStatus: 1,
		PollTimeout:     twait.DefaultTimeout,
		TargetSHA:       g.SHA,
	}
	err := twait.UntilPipelineRunCreated(ctx, g.Cnx.Clients, waitOpts)
	assert.NilError(t, err)

	// Let the PipelineRun start running before canceling
	time.Sleep(2 * time.Second)

	_, _, err = g.Provider.Client().Issues.CreateComment(ctx,
		g.Options.Organization,
		g.Options.Repo,
		g.PRNumber,
		&github.IssueComment{Body: github.Ptr("/cancel")})
	assert.NilError(t, err)

	// Wait for the cancellation to be processed
	err = twait.UntilPipelineRunHasReason(ctx, g.Cnx.Clients, tektonv1.PipelineRunReasonCancelled, waitOpts)
	assert.NilError(t, err)
}
