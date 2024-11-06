//go:build e2e
// +build e2e

package test

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/google/go-github/v66/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/opscomments"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/cctx"
	tgitlab "github.com/openshift-pipelines/pipelines-as-code/test/pkg/gitlab"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/payload"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/scm"
	twait "github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	"github.com/tektoncd/pipeline/pkg/names"
	clientGitlab "github.com/xanzy/go-gitlab"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGitlabMergeRequest(t *testing.T) {
	targetNS := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-ns")
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

	err = tgitlab.CreateCRD(ctx, projectinfo, runcnx, targetNS, nil)
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
	scm.PushFilesToRefGit(t, scmOpts, entries)

	runcnx.Clients.Log.Infof("Branch %s has been created and pushed with files", targetRefName)
	mrTitle := "TestMergeRequest - " + targetRefName
	mrID, err := tgitlab.CreateMR(glprovider.Client, opts.ProjectID, targetRefName, projectinfo.DefaultBranch, mrTitle)
	assert.NilError(t, err)
	runcnx.Clients.Log.Infof("MergeRequest %s/-/merge_requests/%d has been created", projectinfo.WebURL, mrID)
	defer tgitlab.TearDown(ctx, t, runcnx, glprovider, mrID, targetRefName, targetNS, opts.ProjectID)

	// updating labels to test if we skip them, this used to create multiple PRs
	_, _, err = glprovider.Client.MergeRequests.UpdateMergeRequest(opts.ProjectID, mrID, &clientGitlab.UpdateMergeRequestOptions{
		Labels: &clientGitlab.LabelOptions{"hello-label"},
	})
	assert.NilError(t, err)

	// Send another Push to make an update and make sure we react to it
	entries, err = payload.GetEntries(map[string]string{
		"hello-world.yaml": "testdata/pipelinerun.yaml",
	}, targetNS, projectinfo.DefaultBranch,
		triggertype.PullRequest.String(), map[string]string{})
	assert.NilError(t, err)
	scmOpts.BaseRefName = targetRefName
	scm.PushFilesToRefGit(t, scmOpts, entries)

	sopt := twait.SuccessOpt{
		Title:           mrTitle,
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

	runcnx.Clients.Log.Infof("Sending /retest comment on MergeRequest %s/-/merge_requests/%d", projectinfo.WebURL, mrID)
	_, _, err = glprovider.Client.Notes.CreateMergeRequestNote(opts.ProjectID, mrID, &clientGitlab.CreateMergeRequestNoteOptions{
		Body: clientGitlab.Ptr("/retest"),
	})
	assert.NilError(t, err)
	sopt = twait.SuccessOpt{
		Title:           commitTitle,
		OnEvent:         opscomments.RetestAllCommentEventType.String(),
		TargetNS:        targetNS,
		NumberofPRMatch: 5, // this is the max we get in repos status
		SHA:             "",
	}
	runcnx.Clients.Log.Info("Checking that PAC has posted successful comments for all PR that has been tested")
	twait.Succeeded(ctx, t, runcnx, opts, sopt)

	notes, _, err := glprovider.Client.Notes.ListMergeRequestNotes(opts.ProjectID, mrID, nil)
	assert.NilError(t, err)
	successCommentsPost := 0
	for _, n := range notes {
		if successRegexp.MatchString(n.Body) {
			successCommentsPost++
		}
	}
	// we get 2 PRS initially, 2 prs from the push update and 2 prs from the /retest == 6
	assert.Equal(t, 6, successCommentsPost)
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

	projectinfo, resp, err := glprovider.Client.Projects.GetProject(opts.ProjectID, nil)
	assert.NilError(t, err)
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		t.Errorf("Repository %s not found in %s", opts.Organization, opts.Repo)
	}

	err = tgitlab.CreateCRD(ctx, projectinfo, runcnx, targetNS, nil)
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
	scm.PushFilesToRefGit(t, scmOpts, entries)

	runcnx.Clients.Log.Infof("Branch %s has been created and pushed with files", targetRefName)
	mrTitle := "TestMergeRequest - " + targetRefName
	mrID, err := tgitlab.CreateMR(glprovider.Client, opts.ProjectID, targetRefName, projectinfo.DefaultBranch, mrTitle)
	assert.NilError(t, err)
	runcnx.Clients.Log.Infof("MergeRequest %s/-/merge_requests/%d has been created", projectinfo.WebURL, mrID)
	defer tgitlab.TearDown(ctx, t, runcnx, glprovider, mrID, targetRefName, targetNS, opts.ProjectID)

	note, _, err := glprovider.Client.Notes.CreateMergeRequestNote(opts.ProjectID, mrID, &clientGitlab.CreateMergeRequestNoteOptions{
		Body: github.String(triggerComment),
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
	mr, _, err := glprovider.Client.MergeRequests.GetMergeRequest(opts.ProjectID, mrID, nil)
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

func TestGitlabIssueGitopsComment(t *testing.T) {
	targetNS := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-ns")
	ctx := context.Background()
	runcnx, opts, glprovider, err := tgitlab.Setup(ctx)
	assert.NilError(t, err)
	ctx, err = cctx.GetControllerCtxInfo(ctx, runcnx)
	assert.NilError(t, err)
	runcnx.Clients.Log.Info("Testing Gitlabs test/retest comments")
	projectinfo, resp, err := glprovider.Client.Projects.GetProject(opts.ProjectID, nil)
	assert.NilError(t, err)
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		t.Errorf("Repository %s not found in %s", opts.Organization, opts.Repo)
	}

	err = tgitlab.CreateCRD(ctx, projectinfo, runcnx, targetNS, nil)
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
	scm.PushFilesToRefGit(t, scmOpts, entries)

	runcnx.Clients.Log.Infof("Branch %s has been created and pushed with files", targetRefName)
	mrID, err := tgitlab.CreateMR(glprovider.Client, opts.ProjectID, targetRefName, projectinfo.DefaultBranch, mrTitle)
	assert.NilError(t, err)
	runcnx.Clients.Log.Infof("MergeRequest %s/-/merge_requests/%d has been created", projectinfo.WebURL, mrID)
	defer tgitlab.TearDown(ctx, t, runcnx, glprovider, mrID, targetRefName, targetNS, opts.ProjectID)

	_, _, err = glprovider.Client.Notes.CreateMergeRequestNote(opts.ProjectID, mrID, &clientGitlab.CreateMergeRequestNoteOptions{
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

// Local Variables:
// compile-command: "go test -tags=e2e -v -run ^TestGitlabMergeRequest$"
// End:
