//go:build e2e

package test

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v81/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	tgithub "github.com/openshift-pipelines/pipelines-as-code/test/pkg/github"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	twait "github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGithubPullRequestSingleCommentStrategyWebhook(t *testing.T) {
	if os.Getenv("TEST_GITHUB_REPO_OWNER_WEBHOOK") == "" {
		t.Skip("TEST_GITHUB_REPO_OWNER_WEBHOOK is not set")
	}
	ctx := context.Background()

	g := &tgithub.PRTest{
		Label:     "Github Single Comment Strategy Webhook",
		YamlFiles: []string{"testdata/pipelinerun.yaml"},
		Webhook:   true,
	}

	g.Options = options.E2E{
		Organization:  os.Getenv("TEST_GITHUB_REPO_OWNER_WEBHOOK"),
		Repo:          os.Getenv("TEST_GITHUB_REPO_NAME_WEBHOOK"),
		DirectWebhook: true,
		Settings: v1alpha1.Settings{
			Github: &v1alpha1.GithubSettings{
				CommentStrategy: "update",
			},
		},
	}

	g.RunPullRequest(ctx, t)
	defer g.TearDown(ctx, t)

	g.Cnx.Clients.Log.Infof("Waiting for PipelineRun to complete")
	waitOpts := twait.Opts{
		RepoName:    g.TargetNamespace,
		Namespace:   g.TargetNamespace,
		PollTimeout: twait.DefaultTimeout,
		TargetSHA:   g.SHA,
	}
	err := twait.UntilPipelineRunCompleted(ctx, g.Cnx.Clients, waitOpts)
	assert.NilError(t, err)

	g.Cnx.Clients.Log.Infof("Verifying status comment created")
	comments, _, err := g.Provider.Client().Issues.ListComments(
		ctx, g.Options.Organization, g.Options.Repo, g.PRNumber,
		&github.IssueListCommentsOptions{})
	assert.NilError(t, err)

	statusComments := []*github.IssueComment{}
	for _, comment := range comments {
		if strings.Contains(comment.GetBody(), "<!-- pac-status-") {
			statusComments = append(statusComments, comment)
		}
	}

	assert.Assert(t, len(statusComments) == 1, "Expected 1 status comment, got nothing")

	initialCommentID := statusComments[0].GetID()
	initialCommentBody := statusComments[0].GetBody()
	g.Cnx.Clients.Log.Infof("Found initial comment with ID: %d", initialCommentID)

	assert.Assert(t, strings.Contains(initialCommentBody, "Success"), "Comment should show pipeline status")

	labelSelector := fmt.Sprintf("%s=%s,%s=%d", keys.SHA, g.SHA, keys.PullRequest, g.PRNumber)

	// Get the first PipelineRun to use its name in the log pattern search.
	pruns, err := g.Cnx.Clients.Tekton.TektonV1().PipelineRuns(g.TargetNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	assert.NilError(t, err)
	assert.Assert(t, len(pruns.Items) >= 1, "Expected at least 1 PipelineRun")
	firstPRun := pruns.Items[0]
	firstPRunName := firstPRun.GetName()
	originalPipelineName := firstPRun.GetAnnotations()[keys.OriginalPRName]
	assert.Assert(t, originalPipelineName != "", "PipelineRun should have original-prname annotation")

	// Get the PAC install namespace to search for watcher logs
	pacNS, _, err := params.GetInstallLocation(ctx, g.Cnx)
	assert.NilError(t, err)
	ctxWithNS := info.StoreNS(ctx, pacNS)

	// Wait for the watcher to patch the PipelineRun with the cached comment ID
	// Use a pattern specific to this PipelineRun name to avoid matching other runs
	g.Cnx.Clients.Log.Infof("Waiting for watcher to cache comment ID for PipelineRun %s", firstPRunName)
	logLines := int64(100)
	cacheLogPattern := regexp.MustCompile(fmt.Sprintf(`patched pipelinerun with cache comment ID.*%s`, originalPipelineName))
	err = twait.RegexpMatchingInPACLog(ctxWithNS, g.Cnx, *cacheLogPattern, 10, "watcher", &logLines)
	if err != nil {
		g.Cnx.Clients.Log.Warnf("Watcher log pattern not found (may already be processed): %v", err)
	}

	g.Cnx.Clients.Log.Infof("Pushing new commit to trigger pipeline again")
	newSHA, err := tgithub.PushFilesToExistingBranch(ctx, g.Provider.Client(),
		"Trigger second pipeline run", g.SHA, g.TargetRefName,
		g.Options.Organization, g.Options.Repo,
		map[string]string{"dummy.txt": fmt.Sprintf("triggered at %s", time.Now().String())})
	assert.NilError(t, err)
	g.Cnx.Clients.Log.Infof("Pushed new commit %s", newSHA)

	g.Cnx.Clients.Log.Infof("Waiting for PipelineRun with new SHA %s to complete", newSHA)
	waitOpts.TargetSHA = newSHA
	err = twait.UntilPipelineRunCompleted(ctx, g.Cnx.Clients, waitOpts)
	assert.NilError(t, err)

	time.Sleep(5 * time.Second)

	g.Cnx.Clients.Log.Infof("Verifying comment was updated, not recreated")
	comments, _, err = g.Provider.Client().Issues.ListComments(
		ctx, g.Options.Organization, g.Options.Repo, g.PRNumber,
		&github.IssueListCommentsOptions{})
	assert.NilError(t, err)

	statusComments = []*github.IssueComment{}
	for _, comment := range comments {
		if strings.Contains(comment.GetBody(), "<!-- pac-status-") {
			statusComments = append(statusComments, comment)
		}
	}

	assert.Assert(t, len(statusComments) >= 1,
		"After rerun, expected at least 1 status comment, got %d.",
		len(statusComments))

	var updatedCommentBody string
	for _, comment := range statusComments {
		if comment.GetID() == initialCommentID && updatedCommentBody != initialCommentBody {
			updatedCommentBody = comment.GetBody()
			g.Cnx.Clients.Log.Infof("Initial comment (ID %d) found after rerun", initialCommentID)
			g.Cnx.Clients.Log.Infof("Comment was updated, body changed from initial")
			break
		}
	}

	assert.Assert(t, updatedCommentBody != "",
		"Initial comment (ID %d) should still exist after retest. Found comment IDs: %v",
		initialCommentID, func() []int64 {
			ids := make([]int64, len(statusComments))
			for i, c := range statusComments {
				ids[i] = c.GetID()
			}
			return ids
		}())

	assert.Assert(t, strings.Contains(updatedCommentBody, newSHA),
		"Updated comment should contain new SHA")

	g.Cnx.Clients.Log.Infof("Single comment strategy working correctly")
}

func TestGithubPullRequestDefaultCommentStrategy(t *testing.T) {
	if os.Getenv("NIGHTLY_E2E_TEST") != "true" {
		t.Skip("Skipping test since only enabled for nightly")
	}
	ctx := context.Background()

	g := &tgithub.PRTest{
		Label:     "Github Default Comment Strategy",
		YamlFiles: []string{"testdata/pipelinerun.yaml"},
		Webhook:   true,
	}

	g.Options = options.E2E{
		Organization:  os.Getenv("TEST_GITHUB_REPO_OWNER_WEBHOOK"),
		Repo:          os.Getenv("TEST_GITHUB_REPO_NAME_WEBHOOK"),
		DirectWebhook: true,
		Settings:      v1alpha1.Settings{},
	}

	g.RunPullRequest(ctx, t)
	defer g.TearDown(ctx, t)

	waitOpts := twait.Opts{
		RepoName:        g.TargetNamespace,
		Namespace:       g.TargetNamespace,
		MinNumberStatus: 1,
		PollTimeout:     twait.DefaultTimeout,
		TargetSHA:       g.SHA,
	}
	_, err := twait.UntilRepositoryUpdated(ctx, g.Cnx.Clients, waitOpts)
	assert.NilError(t, err)

	comments, _, err := g.Provider.Client().Issues.ListComments(
		ctx, g.Options.Organization, g.Options.Repo, g.PRNumber,
		&github.IssueListCommentsOptions{})
	assert.NilError(t, err)

	initialCommentCount := 0
	for _, comment := range comments {
		body := comment.GetBody()
		if strings.Contains(body, "PipelineRun") {
			initialCommentCount++
		}
	}

	g.Cnx.Clients.Log.Infof("Initial comment count: %d", initialCommentCount)
	assert.Assert(t, initialCommentCount > 0, "Should have at least one comment")

	g.Cnx.Clients.Log.Infof("Pushing new commit to trigger pipeline again")
	newSHA, err := tgithub.PushFilesToExistingBranch(ctx, g.Provider.Client(),
		"Trigger second pipeline run", g.SHA, g.TargetRefName,
		g.Options.Organization, g.Options.Repo,
		map[string]string{"dummy.txt": fmt.Sprintf("triggered at %s", time.Now().String())})
	assert.NilError(t, err)
	g.Cnx.Clients.Log.Infof("Pushed new commit %s", newSHA)

	g.Cnx.Clients.Log.Infof("Waiting for PipelineRun with new SHA %s to complete", newSHA)
	waitOpts.TargetSHA = newSHA
	err = twait.UntilPipelineRunCompleted(ctx, g.Cnx.Clients, waitOpts)
	assert.NilError(t, err)

	time.Sleep(5 * time.Second)

	commentsAfterRetest, _, err := g.Provider.Client().Issues.ListComments(
		ctx, g.Options.Organization, g.Options.Repo, g.PRNumber,
		&github.IssueListCommentsOptions{})
	assert.NilError(t, err)

	finalCommentCount := 0
	for _, comment := range commentsAfterRetest {
		body := comment.GetBody()
		if strings.Contains(body, "PipelineRun") {
			finalCommentCount++
		}
	}

	g.Cnx.Clients.Log.Infof("Final comment count: %d", finalCommentCount)

	assert.Assert(t, finalCommentCount > initialCommentCount,
		"Default behaviour should create new comments, not update. Expected more than %d comments, got %d",
		initialCommentCount, finalCommentCount)

	g.Cnx.Clients.Log.Infof("Default behavior verified: creates new comments as expected")
}
