//go:build e2e
// +build e2e

package test

import (
	"context"
	"net/http"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/opscomments"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/cctx"
	tgitlab "github.com/openshift-pipelines/pipelines-as-code/test/pkg/gitlab"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/payload"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/scm"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/names"
	gitlab "gitlab.com/gitlab-org/api/client-go"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGitlabGitOpsCommandTestOnPush(t *testing.T) {
	targetNs := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-ns")
	ctx := context.Background()
	runcnx, opts, glprovider, err := tgitlab.Setup(ctx)
	assert.NilError(t, err)
	ctx, err = cctx.GetControllerCtxInfo(ctx, runcnx)
	assert.NilError(t, err)
	runcnx.Clients.Log.Info("Testing with Gitlab")
	projectinfo, resp, err := glprovider.Client.Projects.GetProject(opts.ProjectID, nil)
	assert.NilError(t, err)
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		t.Errorf("Repository %s not found in %s", opts.Organization, opts.Repo)
	}

	err = tgitlab.CreateCRD(ctx, projectinfo, runcnx, targetNs, nil)
	assert.NilError(t, err)

	entries, err := payload.GetEntries(map[string]string{
		".tekton/pipelinerun-on-push.yaml": "testdata/pipelinerun-on-push.yaml",
	}, targetNs, targetNs, triggertype.Push.String(), map[string]string{})
	assert.NilError(t, err)

	title := "Test GitOps Commands on Push - " + targetNs
	gitCloneURL, err := scm.MakeGitCloneURL(projectinfo.WebURL, opts.UserName, opts.Password)
	assert.NilError(t, err)
	scmOpts := &scm.Opts{
		GitURL:        gitCloneURL,
		Log:           runcnx.Clients.Log,
		WebURL:        projectinfo.WebURL,
		TargetRefName: targetNs,
		BaseRefName:   projectinfo.DefaultBranch,
		CommitTitle:   title,
	}
	_ = scm.PushFilesToRefGit(t, scmOpts, entries)
	runcnx.Clients.Log.Infof("Branch %s has been created and pushed with files", targetNs)
	defer tgitlab.TearDown(ctx, t, runcnx, glprovider, -1, targetNs, targetNs, opts.ProjectID)

	branch, _, err := glprovider.Client.Branches.GetBranch(opts.ProjectID, targetNs)
	assert.NilError(t, err)

	waitOpts := wait.Opts{
		RepoName:        targetNs,
		Namespace:       targetNs,
		MinNumberStatus: 1,
		PollTimeout:     wait.DefaultTimeout,
		TargetSHA:       branch.Commit.ID,
	}

	err = wait.UntilPipelineRunCreated(ctx, runcnx.Clients, waitOpts)
	assert.NilError(t, err)

	commentOpts := &gitlab.PostCommitCommentOptions{
		Note: gitlab.Ptr("/test branch:" + targetNs),
	}
	cc, _, err := glprovider.Client.Commits.PostCommitComment(opts.ProjectID, branch.Commit.ID, commentOpts)
	assert.NilError(t, err)
	runcnx.Clients.Log.Infof("Commit comment %s has been created", cc.Note)

	sopt := wait.SuccessOpt{
		Title:           title,
		OnEvent:         opscomments.TestSingleCommentEventType.String(),
		TargetNS:        targetNs,
		NumberofPRMatch: 2,
	}
	wait.Succeeded(ctx, t, runcnx, opts, sopt)
	prsNew, err := runcnx.Clients.Tekton.TektonV1().PipelineRuns(targetNs).List(ctx, metav1.ListOptions{})
	assert.NilError(t, err)
	assert.Assert(t, len(prsNew.Items) == 2)
}

func TestGitlabGitOpsCommandCancelOnPush(t *testing.T) {
	targetNs := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-ns")
	ctx := context.Background()
	runcnx, opts, glprovider, err := tgitlab.Setup(ctx)
	assert.NilError(t, err)
	ctx, err = cctx.GetControllerCtxInfo(ctx, runcnx)
	assert.NilError(t, err)
	runcnx.Clients.Log.Info("Testing with Gitlab")
	projectinfo, resp, err := glprovider.Client.Projects.GetProject(opts.ProjectID, nil)
	assert.NilError(t, err)
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		t.Errorf("Repository %s not found in %s", opts.Organization, opts.Repo)
	}

	err = tgitlab.CreateCRD(ctx, projectinfo, runcnx, targetNs, nil)
	assert.NilError(t, err)

	entries, err := payload.GetEntries(map[string]string{
		".tekton/pipelinerun-on-push.yaml": "testdata/pipelinerun-on-push.yaml",
	}, targetNs, targetNs, triggertype.Push.String(), map[string]string{})
	assert.NilError(t, err)

	title := "Test GitOps Commands on Push - " + targetNs
	gitCloneURL, err := scm.MakeGitCloneURL(projectinfo.WebURL, opts.UserName, opts.Password)
	assert.NilError(t, err)
	scmOpts := &scm.Opts{
		GitURL:        gitCloneURL,
		Log:           runcnx.Clients.Log,
		WebURL:        projectinfo.WebURL,
		TargetRefName: targetNs,
		BaseRefName:   projectinfo.DefaultBranch,
		CommitTitle:   title,
	}
	_ = scm.PushFilesToRefGit(t, scmOpts, entries)
	runcnx.Clients.Log.Infof("Branch %s has been created and pushed with files", targetNs)
	defer tgitlab.TearDown(ctx, t, runcnx, glprovider, -1, targetNs, targetNs, opts.ProjectID)

	branch, _, err := glprovider.Client.Branches.GetBranch(opts.ProjectID, targetNs)
	assert.NilError(t, err)

	commentOpts := &gitlab.PostCommitCommentOptions{
		Note: gitlab.Ptr("/test branch:" + targetNs),
	}
	cc, _, err := glprovider.Client.Commits.PostCommitComment(opts.ProjectID, branch.Commit.ID, commentOpts)
	assert.NilError(t, err)
	runcnx.Clients.Log.Infof("Commit comment %s has been created", cc.Note)

	numberOfStatus := 2
	waitOpts := wait.Opts{
		RepoName:        targetNs,
		Namespace:       targetNs,
		MinNumberStatus: numberOfStatus,
		PollTimeout:     wait.DefaultTimeout,
		TargetSHA:       branch.Commit.ID,
	}

	err = wait.UntilPipelineRunCreated(ctx, runcnx.Clients, waitOpts)
	assert.NilError(t, err)

	prsNew, err := runcnx.Clients.Tekton.TektonV1().PipelineRuns(targetNs).List(ctx, metav1.ListOptions{})
	assert.NilError(t, err)
	assert.Assert(t, len(prsNew.Items) == numberOfStatus)

	commentOpts = &gitlab.PostCommitCommentOptions{
		Note: gitlab.Ptr("/cancel pipelinerun-on-push branch:" + targetNs),
	}
	cc, _, err = glprovider.Client.Commits.PostCommitComment(opts.ProjectID, branch.Commit.ID, commentOpts)
	assert.NilError(t, err)
	runcnx.Clients.Log.Infof("Commit comment %s has been created", cc.Note)

	err = wait.UntilPipelineRunHasReason(ctx, runcnx.Clients, v1.PipelineRunReasonCancelled, waitOpts)
	assert.NilError(t, err)
}
