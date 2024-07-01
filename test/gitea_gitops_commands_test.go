//go:build e2e

package test

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/opscomments"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/cctx"
	tgitea "github.com/openshift-pipelines/pipelines-as-code/test/pkg/gitea"
	pacrepo "github.com/openshift-pipelines/pipelines-as-code/test/pkg/repository"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/secret"
	twait "github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	"github.com/tektoncd/pipeline/pkg/names"
	"gotest.tools/v3/assert"
)

func TestGiteaCancelRun(t *testing.T) {
	topts := &tgitea.TestOpts{
		TargetEvent: triggertype.PullRequest.String(),
		YAMLFiles: map[string]string{
			".tekton/pr.yaml": "testdata/pipelinerun_long_running.yaml",
		},
		ExpectEvents: false,
	}
	_, f := tgitea.TestPR(t, topts)
	defer f()
	// let pipelineRun start and then cancel it
	time.Sleep(time.Second * 2)
	tgitea.PostCommentOnPullRequest(t, topts, "/cancel")

	waitOpts := twait.Opts{
		RepoName:        topts.TargetNS,
		Namespace:       topts.TargetNS,
		MinNumberStatus: 1,
		PollTimeout:     twait.DefaultTimeout,
		TargetSHA:       topts.PullRequest.Head.Sha,
	}
	_, err := twait.UntilRepositoryUpdated(context.Background(), topts.ParamsRun.Clients, waitOpts)
	assert.Error(t, err, "pipelinerun has failed")

	tgitea.CheckIfPipelineRunsCancelled(t, topts)
}

// TestGiteaOnCommentAnnotation test custom annotations for gitops comment.
func TestGiteaOnCommentAnnotation(t *testing.T) {
	var err error
	ctx := context.Background()
	topts := &tgitea.TestOpts{
		TargetRefName: names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-test"),
		RepoCRParams: &[]v1alpha1.Params{
			{
				Name:  "custom1",
				Value: "foo",
			},
			{
				Name:  "custom2",
				Value: "bar",
			},
			{
				Name:  "custom3",
				Value: "moto",
			},
		},
	}
	triggerComment := "/hello-world"
	topts.TargetNS = topts.TargetRefName
	topts.ParamsRun, topts.Opts, topts.GiteaCNX, err = tgitea.Setup(ctx)
	assert.NilError(t, err, fmt.Errorf("cannot do gitea setup: %w", err))
	ctx, err = cctx.GetControllerCtxInfo(ctx, topts.ParamsRun)
	assert.NilError(t, err)
	assert.NilError(t, pacrepo.CreateNS(ctx, topts.TargetNS, topts.ParamsRun))
	assert.NilError(t, secret.Create(ctx, topts.ParamsRun, map[string]string{"secret": "SHHHHHHH"}, topts.TargetNS, "pac-secret"))
	topts.TargetEvent = triggertype.PullRequest.String()
	topts.YAMLFiles = map[string]string{
		".tekton/on-comment-match.yaml":   "testdata/pipelinerun-on-comment-annotation.yaml",
		".tekton/pull-request-match.yaml": "testdata/pipelinerun.yaml",
	}
	_, f := tgitea.TestPR(t, topts)
	defer f()
	tgitea.WaitForStatus(t, topts, "heads/"+topts.TargetRefName, "", false)

	tgitea.PostCommentOnPullRequest(t, topts, triggerComment)
	// we have two status one for the pull request match and one on comment match from the comment sent
	waitOpts := twait.Opts{
		RepoName:        topts.TargetNS,
		Namespace:       topts.TargetNS,
		MinNumberStatus: 2,
		PollTimeout:     twait.DefaultTimeout,
	}
	_, err = twait.UntilRepositoryUpdated(context.Background(), topts.ParamsRun.Clients, waitOpts)
	assert.NilError(t, err)

	tgitea.PostCommentOnPullRequest(t, topts, triggerComment)
	waitOpts.MinNumberStatus = 3
	// now we should have only 3 status, the last one is the on comment match from the comment sent
	// but should not have matched the pull request ones
	repo, err := twait.UntilRepositoryUpdated(context.Background(), topts.ParamsRun.Clients, waitOpts)
	assert.NilError(t, err)
	assert.Equal(t, len(repo.Status), waitOpts.MinNumberStatus, fmt.Sprintf("should have only %d status", waitOpts.MinNumberStatus))
	assert.Equal(t, *repo.Status[len(repo.Status)-1].EventType, opscomments.OnCommentEventType.String(), "should have a on comment event type")

	last := repo.Status[len(repo.Status)-1]
	err = twait.RegexpMatchingInPodLog(context.Background(), topts.ParamsRun, topts.TargetNS, fmt.Sprintf("tekton.dev/pipelineRun=%s", last.PipelineRunName), "step-task", *regexp.MustCompile(triggerComment), "", 2)
	assert.NilError(t, err)

	tgitea.PostCommentOnPullRequest(t, topts, fmt.Sprintf(`%s revision=main custom1=thisone custom2="another one" custom3="a \"quote\""`, triggerComment))
	waitOpts.MinNumberStatus = 4
	repo, err = twait.UntilRepositoryUpdated(context.Background(), topts.ParamsRun.Clients, waitOpts)
	assert.NilError(t, err)
	assert.Equal(t, len(repo.Status), waitOpts.MinNumberStatus, fmt.Sprintf("should have only %d status", waitOpts.MinNumberStatus))
	assert.Equal(t, *repo.Status[len(repo.Status)-1].EventType, opscomments.OnCommentEventType.String(), "should have a on comment event type")
	// now we should have only 3 status, the last one is the on comment match with an argument redefining the revision which is a standard parameter

	last = repo.Status[len(repo.Status)-1]
	err = twait.RegexpMatchingInPodLog(context.Background(), topts.ParamsRun, topts.TargetNS, fmt.Sprintf("tekton.dev/pipelineRun=%s", last.PipelineRunName), "step-task", regexp.Regexp{}, t.Name(), 2)
	assert.NilError(t, err)
}

// TestGiteaTestPipelineRunExplicitelyWithTestComment will test a pipelinerun
// even if it hasn't matched when we are doing a /test comment.
func TestGiteaTestPipelineRunExplicitelyWithTestComment(t *testing.T) {
	var err error
	ctx := context.Background()
	topts := &tgitea.TestOpts{
		TargetRefName: names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-test"),
		RepoCRParams: &[]v1alpha1.Params{
			{
				Name:  "custom",
				Value: "foo",
			},
		},
	}
	topts.TargetNS = topts.TargetRefName
	topts.ParamsRun, topts.Opts, topts.GiteaCNX, err = tgitea.Setup(ctx)
	assert.NilError(t, err, fmt.Errorf("cannot do gitea setup: %w", err))
	ctx, err = cctx.GetControllerCtxInfo(ctx, topts.ParamsRun)
	assert.NilError(t, err)
	assert.NilError(t, pacrepo.CreateNS(ctx, topts.TargetNS, topts.ParamsRun))
	assert.NilError(t, secret.Create(ctx, topts.ParamsRun, map[string]string{"secret": "SHHHHHHH"}, topts.TargetNS, "pac-secret"))
	topts.TargetEvent = triggertype.PullRequest.String()
	topts.YAMLFiles = map[string]string{
		".tekton/2-pr.yaml":         "testdata/pipelinerun-nomatch.yaml",
		".tekton/1-anotherone.yaml": "testdata/pipelinerun-on-comment-annotation.yaml",
	}
	_, f := tgitea.TestPR(t, topts)
	defer f()
	targetPrName := "no-match"
	tgitea.PostCommentOnPullRequest(t, topts, fmt.Sprintf("/test %s custom=awesome", targetPrName))
	tgitea.WaitForStatus(t, topts, "heads/"+topts.TargetRefName, "", false)
	waitOpts := twait.Opts{
		RepoName:        topts.TargetNS,
		Namespace:       topts.TargetNS,
		MinNumberStatus: 1,
		PollTimeout:     twait.DefaultTimeout,
	}

	repo, err := twait.UntilRepositoryUpdated(context.Background(), topts.ParamsRun.Clients, waitOpts)
	assert.NilError(t, err)
	assert.Equal(t, len(repo.Status), 1, "should have only 1 status")
	assert.Equal(t, *repo.Status[0].EventType, opscomments.TestSingleCommentEventType.String(), "should have a test comment event in status")
	assert.Assert(t, strings.HasPrefix(repo.Status[0].PipelineRunName, targetPrName+"-"),
		"we didn't target the proper pipelinerun, we tested: %s", repo.Status[0].PipelineRunName)

	last := repo.Status[len(repo.Status)-1]
	err = twait.RegexpMatchingInPodLog(context.Background(), topts.ParamsRun, topts.TargetNS, fmt.Sprintf("tekton.dev/pipelineRun=%s", last.PipelineRunName), "step-task", *regexp.MustCompile("custom is awesome"), "", 2)
	assert.NilError(t, err)
}

func TestGiteaRetestAll(t *testing.T) {
	var err error
	ctx := context.Background()
	topts := &tgitea.TestOpts{
		TargetRefName: names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-test"),
	}
	topts.TargetNS = topts.TargetRefName
	topts.ParamsRun, topts.Opts, topts.GiteaCNX, err = tgitea.Setup(ctx)
	assert.NilError(t, err, fmt.Errorf("cannot do gitea setup: %w", err))
	ctx, err = cctx.GetControllerCtxInfo(ctx, topts.ParamsRun)
	assert.NilError(t, err)
	assert.NilError(t, pacrepo.CreateNS(ctx, topts.TargetNS, topts.ParamsRun))
	assert.NilError(t, secret.Create(ctx, topts.ParamsRun, map[string]string{"secret": "SHHHHHHH"}, topts.TargetNS, "pac-secret"))
	topts.TargetEvent = triggertype.PullRequest.String()
	topts.YAMLFiles = map[string]string{
		".tekton/pr.yaml":      "testdata/pipelinerun.yaml",
		".tekton/nomatch.yaml": "testdata/pipelinerun-nomatch.yaml",
	}
	_, f := tgitea.TestPR(t, topts)
	defer f()
	tgitea.PostCommentOnPullRequest(t, topts, "/retest")
	waitOpts := twait.Opts{
		RepoName:        topts.TargetNS,
		Namespace:       topts.TargetNS,
		MinNumberStatus: 2,
		PollTimeout:     twait.DefaultTimeout,
	}

	repo, err := twait.UntilRepositoryUpdated(context.Background(), topts.ParamsRun.Clients, waitOpts)
	assert.NilError(t, err)
	var rt bool
	for _, status := range repo.Status {
		// TODO(chmouel): Revert back to opscomments.RetestAllCommentEventType.String(), as pull_request now due of https://issues.redhat.com/browse/SRVKP-5775
		if *status.EventType == triggertype.PullRequest.String() {
			rt = true
		}
	}
	assert.Assert(t, rt, "should have a retest all comment event in status")
	assert.Equal(t, len(repo.Status), 2, "should have only 2 status")
}
