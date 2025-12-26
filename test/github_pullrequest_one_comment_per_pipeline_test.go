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

	"github.com/google/go-github/v74/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	tgithub "github.com/openshift-pipelines/pipelines-as-code/test/pkg/github"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	twait "github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestGithubPullRequestSingleCommentStrategyWebhook tests the one_per_pipeline comment strategy with webhook.
// This test verifies that:
// 1. Only one comment is created per pipeline (not multiple).
// 2. The comment is updated when the pipeline state changes.
// 3. The comment ID is cached in PipelineRun annotations.
// 4. When retesting, the same comment is updated instead of creating a new one.
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
				CommentStrategy: "one_per_pipeline",
			},
		},
	}

	g.RunPullRequest(ctx, t)
	defer g.TearDown(ctx, t)

	// Wait for pipeline to complete
	g.Cnx.Clients.Log.Infof("Waiting for PipelineRun to complete")
	waitOpts := twait.Opts{
		RepoName:        g.TargetNamespace,
		Namespace:       g.TargetNamespace,
		MinNumberStatus: 1,
		PollTimeout:     twait.DefaultTimeout,
		TargetSHA:       g.SHA,
	}
	_, err := twait.UntilRepositoryUpdated(ctx, g.Cnx.Clients, waitOpts)
	assert.NilError(t, err)

	// Verify only one comment was created.
	g.Cnx.Clients.Log.Infof("Verifying single comment created")
	comments, _, err := g.Provider.Client().Issues.ListComments(
		ctx, g.Options.Organization, g.Options.Repo, g.PRNumber,
		&github.IssueListCommentsOptions{})
	assert.NilError(t, err)

	// Filter for status comments (with pac-status marker)
	statusComments := []*github.IssueComment{}
	for _, comment := range comments {
		if strings.Contains(comment.GetBody(), "<!-- pac-status-") {
			statusComments = append(statusComments, comment)
		}
	}

	assert.Assert(t, len(statusComments) == 1,
		"Expected exactly 1 status comment, got %d. Comments: %+v",
		len(statusComments), statusComments)

	initialCommentID := statusComments[0].GetID()
	g.Cnx.Clients.Log.Infof("Found initial comment with ID: %d", initialCommentID)

	// Verify comment contains the status marker
	commentBody := statusComments[0].GetBody()
	assert.Assert(t, strings.Contains(commentBody, "<!-- pac-status-"),
		"Comment should contain status marker")

	// Verify comment shows completion status
	assert.Assert(t, strings.Contains(commentBody, "Success") ||
		strings.Contains(commentBody, "successfully") ||
		strings.Contains(commentBody, "✅"),
		"Comment should show pipeline status")

	labelSelector := fmt.Sprintf("%s=%s,%s=%d", keys.SHA, g.SHA, keys.PullRequest, g.PRNumber)

	// Get the first PipelineRun to use its name in the log pattern
	pruns, err := g.Cnx.Clients.Tekton.TektonV1().PipelineRuns(g.TargetNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	assert.NilError(t, err)
	assert.Assert(t, len(pruns.Items) >= 1, "Expected at least 1 PipelineRun")
	firstPRun := pruns.Items[0]
	firstPRunName := firstPRun.GetName()

	// Get the PAC install namespace to search for watcher logs
	pacNS, _, err := params.GetInstallLocation(ctx, g.Cnx)
	assert.NilError(t, err)
	ctxWithNS := info.StoreNS(ctx, pacNS)

	// Wait for the watcher to patch the PipelineRun with the cached comment ID
	// Use a pattern specific to this PipelineRun name to avoid matching other runs
	g.Cnx.Clients.Log.Infof("Waiting for watcher to cache comment ID for PipelineRun %s", firstPRunName)
	logLines := int64(100)
	cacheLogPattern := regexp.MustCompile(fmt.Sprintf(`patched pipelinerun with cache comment ID.*%s`, firstPRunName))
	err = twait.RegexpMatchingInPACLog(ctxWithNS, g.Cnx, *cacheLogPattern, 10, "watcher", &logLines)
	if err != nil {
		g.Cnx.Clients.Log.Warnf("Watcher log pattern not found (may already be processed): %v", err)
	}

	// Verify comment ID is cached in PipelineRun annotation
	g.Cnx.Clients.Log.Infof("Verifying comment ID is cached in PipelineRun")
	pruns, err = g.Cnx.Clients.Tekton.TektonV1().PipelineRuns(g.TargetNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	assert.NilError(t, err)
	assert.Assert(t, len(pruns.Items) >= 1, "Expected at least 1 PipelineRun")

	var prun *tektonv1.PipelineRun
	for i := range pruns.Items {
		if pruns.Items[i].GetName() == firstPRunName {
			prun = &pruns.Items[i]
			break
		}
	}
	assert.Assert(t, prun != nil, "Could not find first PipelineRun %s", firstPRunName)
	originalPipelineName := prun.GetAnnotations()[keys.OriginalPRName]
	assert.Assert(t, originalPipelineName != "", "PipelineRun should have original-prname annotation")

	commentIDKey := fmt.Sprintf("%s-%s", keys.StatusCommentID, originalPipelineName)
	cachedCommentID, found := prun.GetAnnotations()[commentIDKey]
	assert.Assert(t, found, "Comment ID should be cached in annotation %s", commentIDKey)
	assert.Assert(t, cachedCommentID != "", "Cached comment ID should not be empty")
	g.Cnx.Clients.Log.Infof("Found cached comment ID in annotation: %s", cachedCommentID)

	// Push a new commit to trigger the pipeline again
	g.Cnx.Clients.Log.Infof("Pushing new commit to trigger pipeline again")
	newSHA, err := tgithub.PushFilesToExistingBranch(ctx, g.Provider.Client(),
		"Trigger second pipeline run", g.SHA, g.TargetRefName,
		g.Options.Organization, g.Options.Repo,
		map[string]string{"dummy.txt": fmt.Sprintf("triggered at %s", time.Now().String())})
	assert.NilError(t, err)
	g.Cnx.Clients.Log.Infof("Pushed new commit %s", newSHA)

	// TODO: theakshaypant: For some reason twice the number of PLRs are getting created with
	// the same manifest. Using sleep to avoid unexpected behaviour.
	time.Sleep(10 * time.Second)

	// Wait for new PipelineRun to be created
	g.Cnx.Clients.Log.Infof("Waiting for retest PipelineRun to be created")
	waitOpts.MinNumberStatus = 2
	_, err = twait.UntilRepositoryUpdated(ctx, g.Cnx.Clients, waitOpts)
	assert.NilError(t, err)

	// Verify still only one comment exists (same ID, updated content)
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

	// Should still have exactly 1 comment, NOT 2
	assert.Assert(t, len(statusComments) == 1,
		"After retest, expected exactly 1 status comment (updated), got %d. "+
			"This means comments are being recreated instead of updated!",
		len(statusComments))

	// Verify it's the SAME comment (same ID) that was updated
	finalCommentID := statusComments[0].GetID()
	assert.Equal(t, initialCommentID, finalCommentID,
		"Comment ID changed! Expected comment to be updated (ID %d), but got new comment (ID %d)",
		initialCommentID, finalCommentID)

	g.Cnx.Clients.Log.Infof("✅ SUCCESS: Same comment was updated (ID %d), not recreated", finalCommentID)

	// Verify comment was actually updated (check update timestamp or content)
	commentBody = statusComments[0].GetBody()
	assert.Assert(t, strings.Contains(commentBody, "<!-- pac-status-"),
		"Updated comment should still contain status marker")

	// Get the list of PipelineRuns to find the latest one
	prunsAfterRetest, err := g.Cnx.Clients.Tekton.TektonV1().PipelineRuns(g.TargetNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	assert.NilError(t, err)
	assert.Assert(t, len(prunsAfterRetest.Items) >= 2, "Expected at least 2 PipelineRuns after second push, got %d", len(prunsAfterRetest.Items))

	// Find the second PipelineRun (the one that is NOT the first one)
	var secondPRun *tektonv1.PipelineRun
	for i := range prunsAfterRetest.Items {
		if prunsAfterRetest.Items[i].GetName() != firstPRunName {
			if secondPRun == nil || prunsAfterRetest.Items[i].CreationTimestamp.After(secondPRun.CreationTimestamp.Time) {
				secondPRun = &prunsAfterRetest.Items[i]
			}
		}
	}
	assert.Assert(t, secondPRun != nil, "Could not find second PipelineRun (expected a PipelineRun different from %s)", firstPRunName)
	secondPRunName := secondPRun.GetName()
	g.Cnx.Clients.Log.Infof("Second PipelineRun: %s (created at %v)", secondPRunName, secondPRun.CreationTimestamp)

	// Verify that the new PipelineRun also has the cached comment ID
	g.Cnx.Clients.Log.Infof("Checking if new PipelineRun %s also has cached comment ID", secondPRunName)

	secondCacheLogPattern := regexp.MustCompile(fmt.Sprintf(`patched pipelinerun with cache comment ID.*%s`, secondPRunName))
	err = twait.RegexpMatchingInPACLog(ctxWithNS, g.Cnx, *secondCacheLogPattern, 10, "watcher", &logLines)
	assert.NilError(t, err)

	// Re-fetch the second PipelineRun to verify the cached comment ID annotation
	prunsAfterRetest, err = g.Cnx.Clients.Tekton.TektonV1().PipelineRuns(g.TargetNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	assert.NilError(t, err)

	// Find the second PipelineRun by name
	for i := range prunsAfterRetest.Items {
		if prunsAfterRetest.Items[i].GetName() == secondPRunName {
			secondPRun = &prunsAfterRetest.Items[i]
			break
		}
	}
	assert.Assert(t, secondPRun != nil, "Could not re-fetch second PipelineRun %s", secondPRunName)

	// Verify the second PipelineRun has the cached comment ID
	newCachedCommentID, found := secondPRun.GetAnnotations()[commentIDKey]
	assert.Assert(t, found, "Second PipelineRun %s should have cached comment ID annotation %s", secondPRunName, commentIDKey)
	assert.Equal(t, cachedCommentID, newCachedCommentID,
		"Both PipelineRuns should reference the same comment ID")
	g.Cnx.Clients.Log.Infof("Second PipelineRun also has cached comment ID: %s", newCachedCommentID)

	g.Cnx.Clients.Log.Infof("Single comment strategy working correctly")
}

// TestGithubPullRequestDefaultCommentStrategy verifies the default behavior of creating new comments.
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

	// Count initial comments
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

	// Push a new commit to trigger the pipeline again
	g.Cnx.Clients.Log.Infof("Pushing new commit to trigger pipeline again")
	newSHA, err := tgithub.PushFilesToExistingBranch(ctx, g.Provider.Client(),
		"Trigger second pipeline run", g.SHA, g.TargetRefName,
		g.Options.Organization, g.Options.Repo,
		map[string]string{"dummy.txt": fmt.Sprintf("triggered at %s", time.Now().String())})
	assert.NilError(t, err)
	g.Cnx.Clients.Log.Infof("Pushed new commit %s", newSHA)

	// TODO: theakshaypant: For some reason twice the number of PLRs are getting created with
	// the same manifest. Using sleep to avoid unexpected behaviour.
	time.Sleep(10 * time.Second)

	// Update SHA for cleanup and waiting
	waitOpts.TargetSHA = newSHA

	waitOpts.MinNumberStatus = 2
	_, err = twait.UntilRepositoryUpdated(ctx, g.Cnx.Clients, waitOpts)
	assert.NilError(t, err)

	// Verify new comment was created
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

	// Should have more comments after retest
	assert.Assert(t, finalCommentCount > initialCommentCount,
		"Default behaviour should create new comments, not update. Expected more than %d comments, got %d",
		initialCommentCount, finalCommentCount)

	g.Cnx.Clients.Log.Infof("✅ Default behavior verified: creates new comments as expected")
}
