//go:build e2e
// +build e2e

package test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/go-github/v35/github"
	pacv1alpha1 "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	tgithub "github.com/openshift-pipelines/pipelines-as-code/test/pkg/github"
	trepo "github.com/openshift-pipelines/pipelines-as-code/test/pkg/repository"
	twait "github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	"github.com/tektoncd/pipeline/pkg/names"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPullRequestRetest(t *testing.T) {
	targetNS := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-ns")
	ctx := context.Background()
	runcnx, opts, ghcnx, err := setup(ctx)
	assert.NilError(t, err)

	entries := map[string]string{
		".tekton/info.yaml": fmt.Sprintf(`---
apiVersion: tekton.dev/v1beta1
kind: PipelineRun
metadata:
  name: pipeline
  annotations:
    pipelinesascode.tekton.dev/target-namespace: "%s"
    pipelinesascode.tekton.dev/on-target-branch: "[%s]"
    pipelinesascode.tekton.dev/on-event: "[%s]"
spec:
  pipelineSpec:
    tasks:
      - name: task
        taskSpec:
          steps:
            - name: task
              image: gcr.io/google-containers/busybox
              command: ["/bin/echo", "HELLOMOTO"]
`, targetNS, mainBranch, pullRequestEvent),
	}

	repoinfo, resp, err := ghcnx.Client.Repositories.Get(ctx, opts.Owner, opts.Repo)
	assert.NilError(t, err)
	if resp != nil && resp.Response.StatusCode == http.StatusNotFound {
		t.Errorf("Repository %s not found in %s", opts.Owner, opts.Repo)
	}

	repository := &pacv1alpha1.Repository{
		ObjectMeta: metav1.ObjectMeta{
			Name: targetNS,
		},
		Spec: pacv1alpha1.RepositorySpec{
			Namespace: targetNS,
			URL:       repoinfo.GetHTMLURL(),
			EventType: pullRequestEvent,
			Branch:    mainBranch,
		},
	}

	err = trepo.CreateNSRepo(ctx, targetNS, runcnx, repository)
	assert.NilError(t, err)

	targetRefName := fmt.Sprintf("refs/heads/%s",
		names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-test"))

	sha, err := tgithub.PushFilesToRef(ctx, ghcnx.Client, "TestRetest - "+targetRefName, repoinfo.GetDefaultBranch(), targetRefName, opts.Owner, opts.Repo, entries)
	assert.NilError(t, err)
	runcnx.Clients.Log.Infof("Commit %s has been created and pushed to %s", sha, targetRefName)
	title := "TestPullRequestRetest on " + targetRefName

	number, err := tgithub.PRCreate(ctx, runcnx, ghcnx, opts.Owner, opts.Repo, targetRefName, repoinfo.GetDefaultBranch(), title)
	assert.NilError(t, err)

	defer tearDown(ctx, t, runcnx, ghcnx, number, targetRefName, targetNS, opts)

	runcnx.Clients.Log.Infof("Waiting for Repository to be updated")

	waitOpts := twait.Opts{
		RepoName:        targetNS,
		Namespace:       targetNS,
		MinNumberStatus: 0,
		PollTimeout:     defaultTimeout,
		TargetSHA:       sha,
	}
	err = twait.UntilRepositoryUpdated(ctx, runcnx.Clients, waitOpts)
	assert.NilError(t, err)

	runcnx.Clients.Log.Infof("Creating /retest in PullRequest")
	_, _, err = ghcnx.Client.Issues.CreateComment(ctx,
		opts.Owner,
		opts.Repo, number,
		&github.IssueComment{Body: github.String("/retest")})
	assert.NilError(t, err)

	runcnx.Clients.Log.Infof("Wait for the second repository update to be updated")
	waitOpts = twait.Opts{
		RepoName:        targetNS,
		Namespace:       targetNS,
		MinNumberStatus: 1,
		PollTimeout:     defaultTimeout,
		TargetSHA:       sha,
	}
	err = twait.UntilRepositoryUpdated(ctx, runcnx.Clients, waitOpts)
	assert.NilError(t, err)

	runcnx.Clients.Log.Infof("Check if we have the repository set as succeeded")
	repo, err := runcnx.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(targetNS).Get(ctx, targetNS, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Assert(t, repo.Status[len(repo.Status)-1].Conditions[0].Status == corev1.ConditionTrue)
}
