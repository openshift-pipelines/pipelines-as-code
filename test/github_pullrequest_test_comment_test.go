//go:build e2e

package test

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/google/go-github/v74/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/opscomments"
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
	g := &tgithub.PRTest{
		Label:            "Github test implicit comment",
		YamlFiles:        []string{"testdata/pipelinerun.yaml", "testdata/pipelinerun-clone.yaml"},
		SecondController: false,
	}
	g.RunPullRequest(ctx, t)
	defer g.TearDown(ctx, t)

	g.Cnx.Clients.Log.Infof("Creating /test in PullRequest")
	_, _, err := g.Provider.Client().Issues.CreateComment(ctx,
		g.Options.Organization,
		g.Options.Repo, g.PRNumber,
		&github.IssueComment{Body: github.Ptr("/test pipeline")})
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
	_, _, err := g.Provider.Client().Issues.CreateComment(ctx, g.Options.Organization, g.Options.Repo, g.PRNumber,
		&github.IssueComment{Body: github.Ptr(triggerComment)})
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
	assert.Equal(t, repo.Status[len(repo.Status)-1].Conditions[0].Status, corev1.ConditionTrue)
	assert.Equal(t, *repo.Status[len(repo.Status)-1].EventType, opscomments.OnCommentEventType.String())
	lastPrName := repo.Status[len(repo.Status)-1].PipelineRunName

	err = twait.RegexpMatchingInPodLog(context.Background(), g.Cnx, g.TargetNamespace, fmt.Sprintf("tekton.dev/pipelineRun=%s", lastPrName), "step-task", *regexp.MustCompile(triggerComment), "", 2)
	assert.NilError(t, err)

	err = twait.RegexpMatchingInPodLog(context.Background(), g.Cnx, g.TargetNamespace, fmt.Sprintf("tekton.dev/pipelineRun=%s", lastPrName), "step-task", *regexp.MustCompile(fmt.Sprintf(
		"The event is %s", opscomments.OnCommentEventType.String())), "", 2)
	assert.NilError(t, err)
}

func TestGithubPullRequestCELEnrichedComment(t *testing.T) {
	ctx := context.Background()
	g := &tgithub.PRTest{
		Label:     "Github CEL enriched comment event",
		YamlFiles: []string{"testdata/pipelinerun-cel-enriched-comment.yaml"},
	}
	g.RunPullRequest(ctx, t)
	defer g.TearDown(ctx, t)

	g.Cnx.Clients.Log.Infof("Creating /test comment on PullRequest to trigger CEL evaluation with enriched PR data")
	_, _, err := g.Provider.Client().Issues.CreateComment(ctx,
		g.Options.Organization,
		g.Options.Repo, g.PRNumber,
		&github.IssueComment{Body: github.Ptr("/test")})
	assert.NilError(t, err)

	g.Cnx.Clients.Log.Infof("Waiting for second PipelineRun to be created from /test comment")
	sopt := twait.SuccessOpt{
		Title:           fmt.Sprintf("Testing %s with Github APPS integration on %s", g.Label, g.TargetNamespace),
		OnEvent:         "test-all-comment",
		TargetNS:        g.TargetNamespace,
		NumberofPRMatch: 2,
	}
	twait.Succeeded(ctx, t, g.Cnx, g.Options, sopt)

	prs, err := g.Cnx.Clients.Tekton.TektonV1().PipelineRuns(g.TargetNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", keys.SHA, g.SHA),
	})
	assert.NilError(t, err)

	// Find the PipelineRun triggered by the /test comment
	var prName string
	for _, pr := range prs.Items {
		if pr.Annotations[keys.EventType] == "test-all-comment" {
			prName = pr.Name
			g.Cnx.Clients.Log.Infof("Found PipelineRun from /test comment: %s (event type: %s)", prName, pr.Annotations[keys.EventType])
			break
		}
	}
	assert.Assert(t, prName != "", "Should find a PipelineRun with event type test-all-comment")

	err = twait.RegexpMatchingInPodLog(ctx, g.Cnx, g.TargetNamespace,
		fmt.Sprintf("tekton.dev/pipelineRun=%s", prName),
		"step-verify-cel-enrichment",
		*regexp.MustCompile("CEL enrichment verified"),
		"", 1)
	assert.NilError(t, err, "Should find CEL enrichment verification in logs")

	err = twait.RegexpMatchingInPodLog(ctx, g.Cnx, g.TargetNamespace,
		fmt.Sprintf("tekton.dev/pipelineRun=%s", prName),
		"step-verify-cel-enrichment",
		*regexp.MustCompile(`PR User:.*\S+`),
		"", 2)
	assert.NilError(t, err, "Should find PR user in logs from enriched body.pull_request data")

	err = twait.RegexpMatchingInPodLog(ctx, g.Cnx, g.TargetNamespace,
		fmt.Sprintf("tekton.dev/pipelineRun=%s", prName),
		"step-verify-cel-enrichment",
		*regexp.MustCompile(fmt.Sprintf("PR Number: %d", g.PRNumber)),
		"", 2)
	assert.NilError(t, err, "Should find PR number in logs from enriched body.pull_request data")
}
