//go:build e2e
// +build e2e

package test

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"testing"
	"time"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"

	"github.com/google/go-github/v74/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/opscomments"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/cctx"
	tgitlab "github.com/openshift-pipelines/pipelines-as-code/test/pkg/gitlab"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/payload"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/scm"
	twait "github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/names"
	clientGitlab "gitlab.com/gitlab-org/api/client-go"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/pkg/apis"
)

func TestGitlabMergeRequest(t *testing.T) {
	targetNS := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-ns")
	ctx := context.Background()
	runcnx, opts, glprovider, err := tgitlab.Setup(ctx)
	assert.NilError(t, err)
	ctx, err = cctx.GetControllerCtxInfo(ctx, runcnx)
	assert.NilError(t, err)
	runcnx.Clients.Log.Info("Testing with Gitlab")

	projectinfo, resp, err := glprovider.Client().Projects.GetProject(opts.ProjectID, nil)
	assert.NilError(t, err)
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		t.Errorf("Repository %s not found in %s", opts.Organization, opts.Repo)
	}

	err = tgitlab.CreateCRD(ctx, projectinfo, runcnx, opts, targetNS, nil)
	assert.NilError(t, err)

	entries, err := payload.GetEntries(map[string]string{
		".tekton/pipelinerun.yaml":       "testdata/pipelinerun.yaml",
		".tekton/pipelinerun-clone.yaml": "testdata/pipelinerun-clone.yaml",
	}, targetNS, projectinfo.DefaultBranch,
		triggertype.PullRequest.String(), map[string]string{})
	assert.NilError(t, err)

	targetRefName := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-test")

	gitCloneURL, err := scm.MakeGitCloneURL(projectinfo.WebURL, opts.UserName, opts.Password)
	assert.NilError(t, err)
	commitTitle := "Committing files from test on " + targetRefName
	scmOpts := &scm.Opts{
		GitURL:        gitCloneURL,
		CommitTitle:   commitTitle,
		Log:           runcnx.Clients.Log,
		WebURL:        projectinfo.WebURL,
		TargetRefName: targetRefName,
		BaseRefName:   projectinfo.DefaultBranch,
	}
	_ = scm.PushFilesToRefGit(t, scmOpts, entries)

	runcnx.Clients.Log.Infof("Branch %s has been created and pushed with files", targetRefName)
	mrTitle := "TestMergeRequest - " + targetRefName
	mrID, err := tgitlab.CreateMR(glprovider.Client(), opts.ProjectID, targetRefName, projectinfo.DefaultBranch, mrTitle)
	assert.NilError(t, err)
	runcnx.Clients.Log.Infof("MergeRequest %s/-/merge_requests/%d has been created", projectinfo.WebURL, mrID)
	defer tgitlab.TearDown(ctx, t, runcnx, glprovider, mrID, targetRefName, targetNS, opts.ProjectID)

	// Send another Push to make an update and make sure we react to it
	entries, err = payload.GetEntries(map[string]string{
		"hello-world.yaml": "testdata/pipelinerun.yaml",
	}, targetNS, projectinfo.DefaultBranch,
		triggertype.PullRequest.String(), map[string]string{})
	assert.NilError(t, err)
	scmOpts.BaseRefName = targetRefName
	_ = scm.PushFilesToRefGit(t, scmOpts, entries)

	sopt := twait.SuccessOpt{
		Title:           commitTitle,
		OnEvent:         "Merge Request",
		TargetNS:        targetNS,
		NumberofPRMatch: 4,
		SHA:             "",
	}
	twait.Succeeded(ctx, t, runcnx, opts, sopt)
	prsNew, err := runcnx.Clients.Tekton.TektonV1().PipelineRuns(targetNS).List(ctx, metav1.ListOptions{})
	assert.NilError(t, err)
	assert.Assert(t, len(prsNew.Items) == 4)

	for i := 0; i < len(prsNew.Items); i++ {
		assert.Equal(t, "Merge Request", prsNew.Items[i].Annotations[keys.EventType])
	}

	// Get the MR to fetch the SHA
	mr, _, err := glprovider.Client().MergeRequests.GetMergeRequest(opts.ProjectID, mrID, nil)
	assert.NilError(t, err)

	// Check GitLab pipelines via API - should have 1 pipeline from normal MR processing
	pipelines, _, err := glprovider.Client().Pipelines.ListProjectPipelines(opts.ProjectID, &clientGitlab.ListProjectPipelinesOptions{
		SHA: &mr.SHA,
	})
	assert.NilError(t, err)
	assert.Assert(t, len(pipelines) == 1, "Expected 1 GitLab pipeline from normal MR processing, got %d", len(pipelines))

	runcnx.Clients.Log.Infof("Sending /test comment on MergeRequest %s/-/merge_requests/%d", projectinfo.WebURL, mrID)
	_, _, err = glprovider.Client().Notes.CreateMergeRequestNote(opts.ProjectID, mrID, &clientGitlab.CreateMergeRequestNoteOptions{
		Body: clientGitlab.Ptr("/test"),
	})
	assert.NilError(t, err)

	sopt = twait.SuccessOpt{
		Title:           commitTitle,
		OnEvent:         opscomments.TestAllCommentEventType.String(),
		TargetNS:        targetNS,
		NumberofPRMatch: 5, // this is the max we get in repos status
		SHA:             mr.SHA,
	}
	twait.Succeeded(ctx, t, runcnx, opts, sopt)
}

func TestGitlabOnLabel(t *testing.T) {
	prName := "on-label"
	targetNS := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-ns")
	ctx := context.Background()
	runcnx, opts, glprovider, err := tgitlab.Setup(ctx)
	assert.NilError(t, err)
	ctx, err = cctx.GetControllerCtxInfo(ctx, runcnx)
	assert.NilError(t, err)
	runcnx.Clients.Log.Info("Testing with Gitlab")

	projectinfo, resp, err := glprovider.Client().Projects.GetProject(opts.ProjectID, nil)
	assert.NilError(t, err)
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		t.Errorf("Repository %s not found in %s", opts.Organization, opts.Repo)
	}

	err = tgitlab.CreateCRD(ctx, projectinfo, runcnx, opts, targetNS, nil)
	assert.NilError(t, err)

	entries, err := payload.GetEntries(map[string]string{
		fmt.Sprintf(".tekton/%s.yaml", prName): "testdata/pipelinerun-on-label.yaml",
	}, targetNS, projectinfo.DefaultBranch,
		triggertype.PullRequest.String(), map[string]string{})
	assert.NilError(t, err)
	targetRefName := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-test")
	gitCloneURL, err := scm.MakeGitCloneURL(projectinfo.WebURL, opts.UserName, opts.Password)
	assert.NilError(t, err)
	commitTitle := "Committing files from test on " + targetRefName
	scmOpts := &scm.Opts{
		GitURL:        gitCloneURL,
		CommitTitle:   commitTitle,
		Log:           runcnx.Clients.Log,
		WebURL:        projectinfo.WebURL,
		TargetRefName: targetRefName,
		BaseRefName:   projectinfo.DefaultBranch,
	}
	scm.PushFilesToRefGit(t, scmOpts, entries)
	runcnx.Clients.Log.Infof("Branch %s has been created and pushed with files", targetRefName)

	mrTitle := "TestMergeRequest - " + targetRefName
	mrID, err := tgitlab.CreateMR(glprovider.Client(), opts.ProjectID, targetRefName, projectinfo.DefaultBranch, mrTitle)
	assert.NilError(t, err)
	runcnx.Clients.Log.Infof("MergeRequest %s/-/merge_requests/%d has been created", projectinfo.WebURL, mrID)
	defer tgitlab.TearDown(ctx, t, runcnx, glprovider, mrID, targetRefName, targetNS, opts.ProjectID)

	runcnx.Clients.Log.Infof("waiting 5 seconds until we make sure nothing happened")
	time.Sleep(5 * time.Second)
	prsNew, err := runcnx.Clients.Tekton.TektonV1().PipelineRuns(targetNS).List(ctx, metav1.ListOptions{})
	assert.NilError(t, err)
	assert.Assert(t, len(prsNew.Items) == 0)

	// now add a Label
	mr, _, err := glprovider.Client().MergeRequests.UpdateMergeRequest(opts.ProjectID, mrID, &clientGitlab.UpdateMergeRequestOptions{
		Labels: &clientGitlab.LabelOptions{"bug"},
	})
	assert.NilError(t, err)

	waitOpts := twait.Opts{
		RepoName:        targetNS,
		Namespace:       targetNS,
		MinNumberStatus: 1,
		PollTimeout:     twait.DefaultTimeout,
		TargetSHA:       mr.SHA,
	}
	repo, err := twait.UntilRepositoryUpdated(ctx, runcnx.Clients, waitOpts)
	assert.NilError(t, err)
	assert.Assert(t, len(repo.Status) > 0)
	assert.Equal(t, *repo.Status[0].EventType, triggertype.PullRequestLabeled.String())
}

func TestGitlabOnComment(t *testing.T) {
	triggerComment := "/hello-world"
	targetNS := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-ns")
	ctx := context.Background()
	runcnx, opts, glprovider, err := tgitlab.Setup(ctx)
	assert.NilError(t, err)
	ctx, err = cctx.GetControllerCtxInfo(ctx, runcnx)
	assert.NilError(t, err)
	runcnx.Clients.Log.Info("Testing Gitlab on Comment matches")

	projectinfo, resp, err := glprovider.Client().Projects.GetProject(opts.ProjectID, nil)
	assert.NilError(t, err)
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		t.Errorf("Repository %s not found in %s", opts.Organization, opts.Repo)
	}

	err = tgitlab.CreateCRD(ctx, projectinfo, runcnx, opts, targetNS, nil)
	assert.NilError(t, err)

	entries, err := payload.GetEntries(map[string]string{
		".tekton/pipelinerun.yaml": "testdata/pipelinerun-on-comment-annotation.yaml",
	}, targetNS, projectinfo.DefaultBranch,
		triggertype.PullRequest.String(), map[string]string{})
	assert.NilError(t, err)

	targetRefName := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-test")
	gitCloneURL, err := scm.MakeGitCloneURL(projectinfo.WebURL, opts.UserName, opts.Password)
	assert.NilError(t, err)
	scmOpts := &scm.Opts{
		GitURL:        gitCloneURL,
		Log:           runcnx.Clients.Log,
		WebURL:        projectinfo.WebURL,
		TargetRefName: targetRefName,
		BaseRefName:   projectinfo.DefaultBranch,
	}
	_ = scm.PushFilesToRefGit(t, scmOpts, entries)

	runcnx.Clients.Log.Infof("Branch %s has been created and pushed with files", targetRefName)
	mrTitle := "TestMergeRequest - " + targetRefName
	mrID, err := tgitlab.CreateMR(glprovider.Client(), opts.ProjectID, targetRefName, projectinfo.DefaultBranch, mrTitle)
	assert.NilError(t, err)
	runcnx.Clients.Log.Infof("MergeRequest %s/-/merge_requests/%d has been created", projectinfo.WebURL, mrID)
	defer tgitlab.TearDown(ctx, t, runcnx, glprovider, mrID, targetRefName, targetNS, opts.ProjectID)

	note, _, err := glprovider.Client().Notes.CreateMergeRequestNote(opts.ProjectID, mrID, &clientGitlab.CreateMergeRequestNoteOptions{
		Body: github.Ptr(triggerComment),
	})
	assert.NilError(t, err)
	runcnx.Clients.Log.Infof("Note %s/-/merge_requests/%d/notes/%d has been created", projectinfo.WebURL, mrID, note.ID)

	sopt := twait.SuccessOpt{
		OnEvent:         opscomments.OnCommentEventType.String(),
		TargetNS:        targetNS,
		NumberofPRMatch: 1,
		Title:           "Committing files from test on " + targetRefName,
	}
	twait.Succeeded(ctx, t, runcnx, opts, sopt)

	// get pull request info
	mr, _, err := glprovider.Client().MergeRequests.GetMergeRequest(opts.ProjectID, mrID, nil)
	assert.NilError(t, err)

	waitOpts := twait.Opts{
		RepoName:        targetNS,
		Namespace:       targetNS,
		MinNumberStatus: 1,
		PollTimeout:     twait.DefaultTimeout,
		TargetSHA:       mr.SHA,
	}
	repo, err := twait.UntilRepositoryUpdated(ctx, runcnx.Clients, waitOpts)
	assert.NilError(t, err)
	runcnx.Clients.Log.Infof("Check if we have the repository set as succeeded")
	assert.Assert(t, repo.Status[len(repo.Status)-1].Conditions[0].Status == corev1.ConditionTrue)
	lastPrName := repo.Status[len(repo.Status)-1].PipelineRunName

	err = twait.RegexpMatchingInPodLog(context.Background(), runcnx, targetNS, fmt.Sprintf("tekton.dev/pipelineRun=%s", lastPrName), "step-task", *regexp.MustCompile(triggerComment), "", 2)
	assert.NilError(t, err)
}

func TestGitlabCancelInProgressOnChange(t *testing.T) {
	targetNS := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-ns")
	ctx := context.Background()
	runcnx, opts, glprovider, err := tgitlab.Setup(ctx)
	assert.NilError(t, err)
	ctx, err = cctx.GetControllerCtxInfo(ctx, runcnx)
	assert.NilError(t, err)
	runcnx.Clients.Log.Info("Testing Gitlab cancel in progress on pr close")
	projectinfo, resp, err := glprovider.Client().Projects.GetProject(opts.ProjectID, nil)
	assert.NilError(t, err)
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		t.Errorf("Repository %s not found in %s", opts.Organization, opts.Repo)
	}

	err = tgitlab.CreateCRD(ctx, projectinfo, runcnx, opts, targetNS, nil)
	assert.NilError(t, err)

	entries, err := payload.GetEntries(map[string]string{
		".tekton/in-progress.yaml": "testdata/pipelinerun-cancel-in-progress.yaml",
	}, targetNS, projectinfo.DefaultBranch,
		triggertype.PullRequest.String(), map[string]string{})
	assert.NilError(t, err)
	targetRefName := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-test")

	gitCloneURL, err := scm.MakeGitCloneURL(projectinfo.WebURL, opts.UserName, opts.Password)
	assert.NilError(t, err)
	mrTitle := "TestCancelInProgress initial commit - " + targetRefName
	scmOpts := &scm.Opts{
		GitURL:        gitCloneURL,
		Log:           runcnx.Clients.Log,
		WebURL:        projectinfo.WebURL,
		TargetRefName: targetRefName,
		BaseRefName:   projectinfo.DefaultBranch,
		CommitTitle:   mrTitle,
	}

	oldSha := scm.PushFilesToRefGit(t, scmOpts, entries)
	runcnx.Clients.Log.Infof("Branch %s has been created and pushed with files", targetRefName)
	mrID, err := tgitlab.CreateMR(glprovider.Client(), opts.ProjectID, targetRefName, projectinfo.DefaultBranch, mrTitle)
	assert.NilError(t, err)
	runcnx.Clients.Log.Infof("MergeRequest %s/-/merge_requests/%d has been created", projectinfo.WebURL, mrID)
	defer tgitlab.TearDown(ctx, t, runcnx, glprovider, mrID, targetRefName, targetNS, opts.ProjectID)

	runcnx.Clients.Log.Infof("Waiting for the pipelinerun to be created")
	originalPipelineWaitOpts := twait.Opts{
		RepoName:        targetNS,
		Namespace:       targetNS,
		MinNumberStatus: 1,
		PollTimeout:     twait.DefaultTimeout,
		TargetSHA:       oldSha,
	}
	err = twait.UntilPipelineRunCreated(ctx, runcnx.Clients, originalPipelineWaitOpts)
	assert.NilError(t, err)

	newEntries := map[string]string{
		"new-file.txt": "plz work",
	}

	changeTitle := "TestCancelInProgress second commit - " + targetRefName
	scmOpts = &scm.Opts{
		GitURL:        gitCloneURL,
		Log:           runcnx.Clients.Log,
		WebURL:        projectinfo.WebURL,
		TargetRefName: targetRefName,
		BaseRefName:   targetRefName,
		CommitTitle:   changeTitle,
	}
	newSha := scm.PushFilesToRefGit(t, scmOpts, newEntries)

	runcnx.Clients.Log.Infof("Waiting for new pipeline to be created")
	newPipelineWaitOpts := twait.Opts{
		RepoName:        targetNS,
		Namespace:       targetNS,
		MinNumberStatus: 1,
		PollTimeout:     twait.DefaultTimeout,
		TargetSHA:       newSha,
	}
	err = twait.UntilPipelineRunCreated(ctx, runcnx.Clients, newPipelineWaitOpts)
	assert.NilError(t, err)

	runcnx.Clients.Log.Infof("Waiting for old pipelinerun to be canceled")
	cancelledErr := twait.UntilPipelineRunHasReason(ctx, runcnx.Clients, v1.PipelineRunReasonCancelled, originalPipelineWaitOpts)
	assert.NilError(t, cancelledErr)
}

func TestGitlabCancelInProgressOnPRClose(t *testing.T) {
	targetNS := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-ns")
	ctx := context.Background()
	runcnx, opts, glprovider, err := tgitlab.Setup(ctx)
	assert.NilError(t, err)
	ctx, err = cctx.GetControllerCtxInfo(ctx, runcnx)
	assert.NilError(t, err)
	runcnx.Clients.Log.Info("Testing Gitlab cancel in progress on pr close")
	projectinfo, resp, err := glprovider.Client().Projects.GetProject(opts.ProjectID, nil)
	assert.NilError(t, err)
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		t.Errorf("Repository %s not found in %s", opts.Organization, opts.Repo)
	}

	err = tgitlab.CreateCRD(ctx, projectinfo, runcnx, opts, targetNS, nil)
	assert.NilError(t, err)

	entries, err := payload.GetEntries(map[string]string{
		".tekton/in-progress.yaml": "testdata/pipelinerun-cancel-in-progress.yaml",
	}, targetNS, projectinfo.DefaultBranch,
		triggertype.PullRequest.String(), map[string]string{})
	assert.NilError(t, err)
	targetRefName := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-test")

	gitCloneURL, err := scm.MakeGitCloneURL(projectinfo.WebURL, opts.UserName, opts.Password)
	assert.NilError(t, err)
	mrTitle := "TestCancelInProgress - " + targetRefName
	scmOpts := &scm.Opts{
		GitURL:        gitCloneURL,
		Log:           runcnx.Clients.Log,
		WebURL:        projectinfo.WebURL,
		TargetRefName: targetRefName,
		BaseRefName:   projectinfo.DefaultBranch,
		CommitTitle:   mrTitle,
	}

	sha := scm.PushFilesToRefGit(t, scmOpts, entries)
	runcnx.Clients.Log.Infof("Branch %s has been created and pushed with files", targetRefName)
	mrID, err := tgitlab.CreateMR(glprovider.Client(), opts.ProjectID, targetRefName, projectinfo.DefaultBranch, mrTitle)
	assert.NilError(t, err)
	runcnx.Clients.Log.Infof("MergeRequest %s/-/merge_requests/%d has been created", projectinfo.WebURL, mrID)
	defer tgitlab.TearDown(ctx, t, runcnx, glprovider, mrID, targetRefName, targetNS, opts.ProjectID)

	runcnx.Clients.Log.Infof("Waiting for the two pipelinerun to be created")
	waitOpts := twait.Opts{
		RepoName:        targetNS,
		Namespace:       targetNS,
		MinNumberStatus: 1,
		PollTimeout:     twait.DefaultTimeout,
		TargetSHA:       sha,
	}
	err = twait.UntilPipelineRunCreated(ctx, runcnx.Clients, waitOpts)
	assert.NilError(t, err)
	_, _, err = glprovider.Client().MergeRequests.UpdateMergeRequest(opts.ProjectID, mrID, &clientGitlab.UpdateMergeRequestOptions{
		StateEvent: clientGitlab.Ptr("close"),
	})
	assert.NilError(t, err)

	err = twait.UntilPipelineRunHasReason(ctx, runcnx.Clients, v1.PipelineRunReasonCancelled, waitOpts)
	assert.NilError(t, err)

	prs, err := runcnx.Clients.Tekton.TektonV1().PipelineRuns(targetNS).List(context.Background(), metav1.ListOptions{})
	assert.NilError(t, err)
	assert.Equal(t, len(prs.Items), 1, "should have only one pipelinerun, but we have: %d", len(prs.Items))
	assert.Equal(t, prs.Items[0].GetStatusCondition().GetCondition(apis.ConditionSucceeded).GetReason(), "Cancelled", "should have been canceled")

	// failing on `true` condition because for cancelled PipelineRun we want `false` condition.
	waitOpts.FailOnRepoCondition = corev1.ConditionTrue
	repo, err := twait.UntilRepositoryUpdated(ctx, runcnx.Clients, waitOpts)
	assert.NilError(t, err)

	laststatus := repo.Status[len(repo.Status)-1]
	assert.Equal(t, "Cancelled", laststatus.Conditions[0].Reason)
}

func TestGitlabIssueGitopsComment(t *testing.T) {
	targetNS := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-ns")
	ctx := context.Background()
	runcnx, opts, glprovider, err := tgitlab.Setup(ctx)
	assert.NilError(t, err)
	ctx, err = cctx.GetControllerCtxInfo(ctx, runcnx)
	assert.NilError(t, err)
	runcnx.Clients.Log.Info("Testing Gitlabs test/retest comments")
	projectinfo, resp, err := glprovider.Client().Projects.GetProject(opts.ProjectID, nil)
	assert.NilError(t, err)
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		t.Errorf("Repository %s not found in %s", opts.Organization, opts.Repo)
	}

	err = tgitlab.CreateCRD(ctx, projectinfo, runcnx, opts, targetNS, nil)
	assert.NilError(t, err)

	entries, err := payload.GetEntries(map[string]string{
		".tekton/no-match.yaml": "testdata/pipelinerun-nomatch.yaml",
	}, targetNS, projectinfo.DefaultBranch,
		triggertype.PullRequest.String(), map[string]string{})
	assert.NilError(t, err)

	targetRefName := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-test")

	gitCloneURL, err := scm.MakeGitCloneURL(projectinfo.WebURL, opts.UserName, opts.Password)
	assert.NilError(t, err)
	mrTitle := "TestMergeRequest - " + targetRefName
	scmOpts := &scm.Opts{
		GitURL:        gitCloneURL,
		Log:           runcnx.Clients.Log,
		WebURL:        projectinfo.WebURL,
		TargetRefName: targetRefName,
		BaseRefName:   projectinfo.DefaultBranch,
		CommitTitle:   mrTitle,
	}
	_ = scm.PushFilesToRefGit(t, scmOpts, entries)

	runcnx.Clients.Log.Infof("Branch %s has been created and pushed with files", targetRefName)
	mrID, err := tgitlab.CreateMR(glprovider.Client(), opts.ProjectID, targetRefName, projectinfo.DefaultBranch, mrTitle)
	assert.NilError(t, err)
	runcnx.Clients.Log.Infof("MergeRequest %s/-/merge_requests/%d has been created", projectinfo.WebURL, mrID)
	defer tgitlab.TearDown(ctx, t, runcnx, glprovider, mrID, targetRefName, targetNS, opts.ProjectID)

	_, _, err = glprovider.Client().Notes.CreateMergeRequestNote(opts.ProjectID, mrID, &clientGitlab.CreateMergeRequestNoteOptions{
		Body: clientGitlab.Ptr("/test no-match"),
	})
	assert.NilError(t, err)
	runcnx.Clients.Log.Infof("Created gitops comment /test no-match to get the no-match tested")

	sopt := twait.SuccessOpt{
		Title:           mrTitle,
		OnEvent:         opscomments.TestSingleCommentEventType.String(),
		TargetNS:        targetNS,
		NumberofPRMatch: 1,
	}
	twait.Succeeded(ctx, t, runcnx, opts, sopt)
}

func TestGitlabDisableCommentsOnMRNotCreated(t *testing.T) {
	targetNS := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-ns")
	ctx := context.Background()
	runcnx, opts, glprovider, err := tgitlab.Setup(ctx)
	assert.NilError(t, err)
	runcnx.Clients.Log.Info("Testing with Gitlab")

	projectinfo, resp, err := glprovider.Client().Projects.GetProject(opts.ProjectID, nil)
	assert.NilError(t, err)
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		t.Fatalf("Repository %s not found in %s", opts.Organization, opts.Repo) // Use Fatalf to stop test on critical error
	}

	settings := v1alpha1.Settings{
		Gitlab: &v1alpha1.GitlabSettings{
			CommentStrategy: "disable_all",
		},
	}
	opts.Settings = settings
	err = tgitlab.CreateCRD(ctx, projectinfo, runcnx, opts, targetNS, nil)
	assert.NilError(t, err)

	entries, err := payload.GetEntries(map[string]string{
		".tekton/pipelinerun.yaml": "testdata/pipelinerun.yaml",
	}, targetNS, projectinfo.DefaultBranch,
		triggertype.PullRequest.String(), map[string]string{})
	assert.NilError(t, err)

	targetRefName := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-test")

	gitCloneURL, err := scm.MakeGitCloneURL(projectinfo.WebURL, opts.UserName, opts.Password)
	assert.NilError(t, err)
	commitTitle := "Committing files from test on " + targetRefName
	scmOpts := &scm.Opts{
		GitURL:        gitCloneURL,
		CommitTitle:   commitTitle,
		Log:           runcnx.Clients.Log,
		WebURL:        projectinfo.WebURL,
		TargetRefName: targetRefName,
		BaseRefName:   projectinfo.DefaultBranch,
	}
	// NEW: Capture the commit SHA returned by the push operation.
	sha := scm.PushFilesToRefGit(t, scmOpts, entries)
	runcnx.Clients.Log.Infof("Commit %s has been created and pushed to branch %s", sha, targetRefName)

	mrTitle := "TestMergeRequest - " + targetRefName
	mrID, err := tgitlab.CreateMR(glprovider.Client(), opts.ProjectID, targetRefName, projectinfo.DefaultBranch, mrTitle)
	assert.NilError(t, err)
	runcnx.Clients.Log.Infof("MergeRequest %s/-/merge_requests/%d has been created", projectinfo.WebURL, mrID)
	defer tgitlab.TearDown(ctx, t, runcnx, glprovider, mrID, targetRefName, targetNS, opts.ProjectID)

	sopt := twait.SuccessOpt{
		Title:           commitTitle,
		OnEvent:         "Merge Request",
		TargetNS:        targetNS,
		NumberofPRMatch: 1,
		SHA:             sha, // NEW: Pass the captured SHA to ensure we wait for the correct PipelineRun
	}
	twait.Succeeded(ctx, t, runcnx, opts, sopt)
	prsNew, err := runcnx.Clients.Tekton.TektonV1().PipelineRuns(targetNS).List(ctx, metav1.ListOptions{})
	assert.NilError(t, err)
	assert.Assert(t, len(prsNew.Items) == 1)

	runcnx.Clients.Log.Infof("Checking status of GitLab pipeline for commit: %s", sha)
	// Define constants for polling
	const pipelineCheckTimeout = 5 * time.Minute
	const pipelineCheckInterval = 10 * time.Second

	var pipeline *clientGitlab.Pipeline
	// Use a polling mechanism to wait for the pipeline to succeed.
	err = wait.PollUntilContextTimeout(ctx, pipelineCheckInterval, pipelineCheckTimeout, true, func(_ context.Context) (bool, error) {
		// Find the pipeline associated with our specific commit SHA
		pipelines, _, listErr := glprovider.Client().Pipelines.ListProjectPipelines(opts.ProjectID, &clientGitlab.ListProjectPipelinesOptions{
			SHA: &sha,
		})
		if listErr != nil {
			return false, listErr // Propagate API errors
		}
		if len(pipelines) == 0 {
			runcnx.Clients.Log.Info("Waiting for pipeline to be created...")
			return false, nil // Pipeline not found yet, continue polling
		}
		if len(pipelines) > 1 {
			// This is unexpected, fail fast
			return false, fmt.Errorf("expected 1 pipeline for SHA %s, but found %d", sha, len(pipelines))
		}

		// Get the latest status of our specific pipeline
		p, _, getErr := glprovider.Client().Pipelines.GetPipeline(opts.ProjectID, pipelines[0].ID)
		if getErr != nil {
			return false, getErr
		}

		runcnx.Clients.Log.Infof("Current pipeline status: %s", p.Status)
		switch p.Status {
		case "success":
			pipeline = p
			return true, nil // Success! Stop polling.
		case "failed", "canceled", "skipped":
			// The pipeline has finished but not successfully.
			return false, fmt.Errorf("pipeline finished with non-success status: %s", p.Status)
		default:
			// The pipeline is still running or pending, continue polling.
			return false, nil
		}
	})
	assert.NilError(t, err, "failed while waiting for GitLab pipeline to succeed")
	assert.Equal(t, "success", pipeline.Status, "The final pipeline status was not 'success'")
	runcnx.Clients.Log.Infof("✅ GitLab pipeline ID %d has succeeded!", pipeline.ID)

	// No comments will be added related to pipelineruns info
	notes, _, err := glprovider.Client().Notes.ListMergeRequestNotes(opts.ProjectID, mrID, nil)

	commentRegexp := regexp.MustCompile(`.*Pipelines as Code CI/*`)
	assert.NilError(t, err)
	successCommentsPost := 0
	for _, n := range notes {
		if commentRegexp.MatchString(n.Body) {
			successCommentsPost++
		}
	}
	// Since Gitlab comment strategy is disabled,
	// no comments will be posted related to PipelineRuns
	assert.Equal(t, 0, successCommentsPost)
}

func TestGitlabMergeRequestOnUpdateAtAndLabelChange(t *testing.T) {
	targetNS := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-ns")
	ctx := context.Background()
	runcnx, opts, glprovider, err := tgitlab.Setup(ctx)
	assert.NilError(t, err)
	ctx, err = cctx.GetControllerCtxInfo(ctx, runcnx)
	assert.NilError(t, err)
	runcnx.Clients.Log.Info("Testing with Gitlab")

	projectinfo, resp, err := glprovider.Client().Projects.GetProject(opts.ProjectID, nil)
	assert.NilError(t, err)
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		t.Errorf("Repository %s not found in %s", opts.Organization, opts.Repo)
	}

	err = tgitlab.CreateCRD(ctx, projectinfo, runcnx, opts, targetNS, nil)
	assert.NilError(t, err)

	entries, err := payload.GetEntries(map[string]string{
		".tekton/pipelinerun.yaml":       "testdata/pipelinerun.yaml",
		".tekton/pipelinerun-clone.yaml": "testdata/pipelinerun-clone.yaml",
	}, targetNS, projectinfo.DefaultBranch,
		triggertype.PullRequest.String(), map[string]string{})
	assert.NilError(t, err)

	targetRefName := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-test")

	gitCloneURL, err := scm.MakeGitCloneURL(projectinfo.WebURL, opts.UserName, opts.Password)
	assert.NilError(t, err)
	commitTitle := "Committing files from test on " + targetRefName
	scmOpts := &scm.Opts{
		GitURL:        gitCloneURL,
		CommitTitle:   commitTitle,
		Log:           runcnx.Clients.Log,
		WebURL:        projectinfo.WebURL,
		TargetRefName: targetRefName,
		BaseRefName:   projectinfo.DefaultBranch,
	}
	_ = scm.PushFilesToRefGit(t, scmOpts, entries)

	runcnx.Clients.Log.Infof("Branch %s has been created and pushed with files", targetRefName)
	mrTitle := "TestMergeRequest - " + targetRefName
	mrID, err := tgitlab.CreateMR(glprovider.Client(), opts.ProjectID, targetRefName, projectinfo.DefaultBranch, mrTitle)
	assert.NilError(t, err)
	runcnx.Clients.Log.Infof("MergeRequest %s/-/merge_requests/%d has been created", projectinfo.WebURL, mrID)
	defer tgitlab.TearDown(ctx, t, runcnx, glprovider, mrID, targetRefName, targetNS, opts.ProjectID)

	sopt := twait.SuccessOpt{
		Title:           commitTitle,
		OnEvent:         "Merge Request",
		TargetNS:        targetNS,
		NumberofPRMatch: 2,
		SHA:             "",
	}
	twait.Succeeded(ctx, t, runcnx, opts, sopt)
	prsNew, err := runcnx.Clients.Tekton.TektonV1().PipelineRuns(targetNS).List(ctx, metav1.ListOptions{})
	assert.NilError(t, err)
	assert.Assert(t, len(prsNew.Items) == 2)

	runcnx.Clients.Log.Infof("Changing Title on MergeRequest %s/-/merge_requests/%d", projectinfo.WebURL, mrID)
	_, _, err = glprovider.Client().MergeRequests.UpdateMergeRequest(opts.ProjectID, mrID, &clientGitlab.UpdateMergeRequestOptions{
		Title: clientGitlab.Ptr("test"),
	})
	assert.NilError(t, err)

	// let's wait 10 secs and check every second that a PipelineRun is created or not.
	for i := 0; i < 10; i++ {
		prs, err := runcnx.Clients.Tekton.TektonV1().PipelineRuns(targetNS).List(ctx, metav1.ListOptions{})
		assert.NilError(t, err)
		assert.Assert(t, len(prs.Items) == 2)
		time.Sleep(1 * time.Second)
	}
}

func TestGitlabMergeRequestValidationErrorsFromFork(t *testing.T) {
	targetNS := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-ns")
	ctx := context.Background()
	runcnx, opts, glprovider, err := tgitlab.Setup(ctx)
	assert.NilError(t, err)
	ctx, err = cctx.GetControllerCtxInfo(ctx, runcnx)
	assert.NilError(t, err)
	runcnx.Clients.Log.Info("Testing GitLab validation error commenting from fork scenario")

	// Get the original project onboarded to PaC
	originalProject, resp, err := glprovider.Client().Projects.GetProject(opts.ProjectID, nil)
	assert.NilError(t, err)
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		t.Errorf("Repository %d not found", opts.ProjectID)
	}

	err = tgitlab.CreateCRD(ctx, originalProject, runcnx, opts, targetNS, nil)
	assert.NilError(t, err)

	// Get an existing fork of the original project
	projectForks, _, err := glprovider.Client().Projects.ListProjectForks(opts.ProjectID, &clientGitlab.ListProjectsOptions{})
	assert.NilError(t, err)

	if len(projectForks) == 0 {
		t.Fatal("No forks available for testing fork scenario. This test requires at least one fork of the project.")
	}

	forkProject := projectForks[0] // Use the first available fork
	runcnx.Clients.Log.Infof("Using existing fork project: %s (ID: %d) from original: %s (ID: %d)",
		forkProject.PathWithNamespace, forkProject.ID, originalProject.PathWithNamespace, originalProject.ID)

	// Commit invalid .tekton files to the fork
	entries, err := payload.GetEntries(map[string]string{
		".tekton/bad-yaml.yaml": "testdata/failures/bad-yaml.yaml",
	}, targetNS, originalProject.DefaultBranch,
		triggertype.PullRequest.String(), map[string]string{})
	assert.NilError(t, err)

	targetRefName := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-fork-test")
	forkCloneURL, err := scm.MakeGitCloneURL(forkProject.WebURL, opts.UserName, opts.Password)
	assert.NilError(t, err)

	commitTitle := "Add invalid .tekton file from fork - " + targetRefName
	scmOpts := &scm.Opts{
		GitURL:        forkCloneURL,
		CommitTitle:   commitTitle,
		Log:           runcnx.Clients.Log,
		WebURL:        forkProject.WebURL,
		TargetRefName: targetRefName,
		BaseRefName:   originalProject.DefaultBranch,
	}
	_ = scm.PushFilesToRefGit(t, scmOpts, entries)
	runcnx.Clients.Log.Infof("Pushed invalid .tekton files to fork branch: %s", targetRefName)

	// Create merge request from fork to original project
	mrTitle := "TestValidationErrorsFromFork - " + targetRefName
	mrOptions := &clientGitlab.CreateMergeRequestOptions{
		Title:        &mrTitle,
		SourceBranch: &targetRefName,
		TargetBranch: &originalProject.DefaultBranch,
		// Create MR on the target project (original), not the source (fork)
		TargetProjectID: &originalProject.ID,
	}

	mr, _, err := glprovider.Client().MergeRequests.CreateMergeRequest(forkProject.ID, mrOptions)
	assert.NilError(t, err)
	runcnx.Clients.Log.Infof("Created merge request from fork to original: %s/-/merge_requests/%d",
		originalProject.WebURL, mr.IID)

	defer func() {
		// Clean up MR and namespace using TearDown
		tgitlab.TearDown(ctx, t, runcnx, glprovider, mr.IID, "", targetNS, originalProject.ID)

		runcnx.Clients.Log.Infof("Deleting branch %s from fork project", targetRefName)
		_, err := glprovider.Client().Branches.DeleteBranch(forkProject.ID, targetRefName)
		if err != nil {
			runcnx.Clients.Log.Warnf("Failed to delete branch from fork: %v", err)
		}
	}()

	runcnx.Clients.Log.Info("Waiting for webhook validation to process MR and post validation comment...")

	maxLoop := 12 // Wait up to 72 seconds for webhook processing
	foundValidationComment := false

	for i := 0; i < maxLoop; i++ {
		notes, _, err := glprovider.Client().Notes.ListMergeRequestNotes(originalProject.ID, mr.IID, nil)
		assert.NilError(t, err)

		for _, note := range notes {
			// Look for the validation error comment that PaC should post via webhook
			if regexp.MustCompile(`.*There are some errors in your PipelineRun template.*`).MatchString(note.Body) &&
				regexp.MustCompile(`.*bad-yaml\.yaml.*yaml validation error.*`).MatchString(note.Body) {
				runcnx.Clients.Log.Info("Found validation error comment on original project's MR!")
				foundValidationComment = true

				// Verify the comment format matches PaC's validation error format
				assert.Assert(t, regexp.MustCompile(`\[!CAUTION\]`).MatchString(note.Body), "Comment should contain caution header")
				break
			}
		}

		if foundValidationComment {
			break
		}

		runcnx.Clients.Log.Infof("Loop %d/%d: Waiting for webhook validation to post comment (testing TargetProjectID fix)", i+1, maxLoop)
		time.Sleep(6 * time.Second)
	}

	assert.Assert(t, foundValidationComment, "Validation error comment should appear on original project's MR. ")
}

// Local Variables:
// compile-command: "go test -tags=e2e -v -run ^TestGitlabMergeRequest$"
// End:
