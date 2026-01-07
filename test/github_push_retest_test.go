//go:build e2e

package test

import (
	"context"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/google/go-github/v74/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/cctx"
	tgithub "github.com/openshift-pipelines/pipelines-as-code/test/pkg/github"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	twait "github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
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
	pruns, err := g.Cnx.Clients.Tekton.TektonV1().PipelineRuns(g.TargetNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", keys.SHA, g.SHA),
	})
	assert.NilError(t, err)
	assert.Equal(t, len(pruns.Items), 0)

	g.Cnx.Clients.Log.Infof("Running ops comment %s as Push comment", opsComment)
	_, _, err = g.Provider.Client().Repositories.CreateComment(ctx,
		g.Options.Organization,
		g.Options.Repo, g.SHA,
		&github.RepositoryComment{Body: github.Ptr(opsComment)})
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

	pruns, err = g.Cnx.Clients.Tekton.TektonV1().PipelineRuns(g.TargetNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", keys.SHA, g.SHA),
	})
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
		"",
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

	pruns, err := g.Cnx.Clients.Tekton.TektonV1().PipelineRuns(g.TargetNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", keys.SHA, g.SHA),
	})
	assert.NilError(t, err)
	assert.Equal(t, len(pruns.Items), 2)

	g.Cnx.Clients.Log.Infof("%s on Push Request", comment)
	_, _, err = g.Provider.Client().Repositories.CreateComment(ctx,
		g.Options.Organization,
		g.Options.Repo, g.SHA,
		&github.RepositoryComment{Body: github.Ptr(comment)})
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

	pruns, err = g.Cnx.Clients.Tekton.TektonV1().PipelineRuns(g.TargetNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", keys.SHA, g.SHA),
	})
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

	pruns, err := g.Cnx.Clients.Tekton.TektonV1().PipelineRuns(g.TargetNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", keys.SHA, g.SHA),
	})
	assert.NilError(t, err)
	assert.Equal(t, len(pruns.Items), 2)

	g.Cnx.Clients.Log.Info("/test pipelinerun-on-push on Push Request before canceling")
	_, _, err = g.Provider.Client().Repositories.CreateComment(ctx,
		g.Options.Organization,
		g.Options.Repo, g.SHA,
		&github.RepositoryComment{Body: github.Ptr("/test pipelinerun-on-push branch:" + g.TargetNamespace)})
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

	// Get the specific PipelineRun name that was created by the /test comment
	prs, err := g.Cnx.Clients.Tekton.TektonV1().PipelineRuns(g.TargetNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", keys.SHA, g.SHA),
	})
	assert.NilError(t, err)

	var targetPRName string
	for _, pr := range prs.Items {
		annotations := pr.GetAnnotations()
		if annotations != nil &&
			annotations[keys.OriginalPRName] == "pipelinerun-on-push" &&
			annotations[keys.EventType] == "test-comment" {
			targetPRName = pr.GetName()
			break
		}
	}
	assert.Assert(t, targetPRName != "", "Could not find the PipelineRun created by /test comment")

	comment := "/cancel pipelinerun-on-push branch:" + g.TargetNamespace
	g.Cnx.Clients.Log.Infof("%s on Push Request", comment)
	_, _, err = g.Provider.Client().Repositories.CreateComment(ctx,
		g.Options.Organization,
		g.Options.Repo, g.SHA,
		&github.RepositoryComment{Body: github.Ptr(comment)})
	assert.NilError(t, err)

	g.Cnx.Clients.Log.Infof("Waiting for PipelineRun %s to be cancelled or skip message", targetPRName)

	// Wait for PipelineRun to be cancelled using existing helper function
	waitOpts.MinNumberStatus = 1 // Looking for at least 1 cancelled PR
	err = twait.UntilPipelineRunHasReason(ctx, g.Cnx.Clients, tektonv1.PipelineRunReasonCancelled, waitOpts)

	if err == nil {
		// PipelineRun was successfully cancelled, verify in Repository status
		g.Cnx.Clients.Log.Infof("PipelineRun %s was cancelled, verifying Repository status", targetPRName)

		// Wait for Repository status to be updated (reconciler updates asynchronously)
		cancelled := false
		lastReason := ""
		for i := 0; i < 10; i++ {
			repo, err := g.Cnx.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(g.TargetNamespace).Get(ctx, g.TargetNamespace, metav1.GetOptions{})
			assert.NilError(t, err)

			for _, status := range repo.Status {
				if status.PipelineRunName == targetPRName &&
					len(status.Conditions) > 0 &&
					(status.Conditions[0].Reason == tektonv1.PipelineRunReasonCancelled.String() ||
						status.Conditions[0].Reason == tektonv1.PipelineRunReasonCancelledRunningFinally.String()) {
					lastReason = status.Conditions[0].Reason
					cancelled = true
					break
				}
				if status.PipelineRunName == targetPRName && len(status.Conditions) > 0 {
					lastReason = status.Conditions[0].Reason
				}
			}

			if cancelled {
				break
			}
			time.Sleep(1 * time.Second)
		}
		assert.Assert(t, cancelled, fmt.Sprintf("Repository status does not show PipelineRun %s as cancelled after 10 retries (last reason: %q)", targetPRName, lastReason))

		// Final verification: ensure we have the expected number of PipelineRuns
		pruns, err := g.Cnx.Clients.Tekton.TektonV1().PipelineRuns(g.TargetNamespace).List(ctx, metav1.ListOptions{})
		assert.NilError(t, err)
		assert.Equal(t, len(pruns.Items), numberOfStatus)
		return
	}

	// If PipelineRun didn't get cancelled, it likely completed before cancel was processed
	// Check for the skip message in controller logs
	g.Cnx.Clients.Log.Infof("PipelineRun may have completed before cancellation, checking logs for skip message")
	numLines := int64(100) // Use a larger buffer to ensure we capture the skip message in busy logs
	reg := regexp.MustCompile(fmt.Sprintf(".*skipping cancelling pipelinerun %s/%s \\(original: .*\\), already done.*",
		g.TargetNamespace, targetPRName))
	err = twait.RegexpMatchingInControllerLog(ctx, g.Cnx, *reg, 10, "controller", &numLines)
	if err != nil {
		t.Errorf("neither a cancelled pipelinerun in repo status or a request to skip the cancellation in the controller log was found: %s", err.Error())
		return
	}

	g.Cnx.Clients.Log.Infof("Found skip message for PipelineRun %s in controller logs", targetPRName)
}
