// +build e2e

package test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"testing"

	"github.com/google/go-github/v35/github"
	pacv1alpha1 "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"
	tgithub "github.com/openshift-pipelines/pipelines-as-code/test/pkg/github"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/payload"
	trepo "github.com/openshift-pipelines/pipelines-as-code/test/pkg/repository"
	twait "github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	"github.com/tektoncd/pipeline/pkg/names"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPullRequestOkToTest(t *testing.T) {
	targetNS := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-ns")
	ctx := context.Background()
	cs, opts, err := setup()
	assert.NilError(t, err)

	entries := map[string]string{
		".tekton/run.yaml": fmt.Sprintf(`---
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

	repoinfo, resp, err := cs.GithubClient.Client.Repositories.Get(ctx, opts.Owner, opts.Repo)
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

	err = trepo.CreateNSRepo(ctx, targetNS, cs, repository)
	assert.NilError(t, err)

	targetRefName := fmt.Sprintf("refs/heads/%s",
		names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-test"))

	sha, err := tgithub.PushFilesToRef(ctx, cs.GithubClient.Client,
		"TestPullRequest - "+targetRefName, repoinfo.GetDefaultBranch(),
		targetRefName,
		opts.Owner,
		opts.Repo,
		entries)
	assert.NilError(t, err)
	cs.Log.Infof("Commit %s has been created and pushed to %s", sha, targetRefName)
	title := "TestPullRequestOkToTest on " + targetRefName
	number, err := tgithub.PRCreate(ctx, cs, opts.Owner, opts.Repo, targetRefName, repoinfo.GetDefaultBranch(), title)
	assert.NilError(t, err)

	defer tearDown(ctx, t, cs, number, targetRefName, targetNS, opts)

	cs.Log.Infof("Waiting for Repository to be updated")
	err = twait.UntilRepositoryUpdated(ctx, cs.PipelineAsCode, targetNS, targetNS, 0, defaultTimeout)
	assert.NilError(t, err)

	runinfo := webvcs.RunInfo{
		BaseBranch:    repoinfo.GetDefaultBranch(),
		DefaultBranch: repoinfo.GetDefaultBranch(),
		HeadBranch:    targetRefName,
		Owner:         opts.Owner,
		Repository:    opts.Repo,
		URL:           repoinfo.GetHTMLURL(),
		SHA:           sha,
		Sender:        opts.Owner,
	}

	installID, err := strconv.ParseInt(os.Getenv("TEST_GITHUB_REPO_INSTALLATION_ID"), 10, 64)
	assert.NilError(t, err)
	event := github.IssueCommentEvent{
		Comment: &github.IssueComment{
			Body: github.String(`/ok-to-test`),
		},
		Installation: &github.Installation{
			ID: &installID,
		},
		Action: github.String("created"),
		Issue: &github.Issue{
			State: github.String("open"),
			PullRequestLinks: &github.PullRequestLinks{
				HTMLURL: github.String(fmt.Sprintf("%s/%s/pull/%d",
					os.Getenv("TEST_GITHUB_API_URL"),
					os.Getenv("TEST_GITHUB_REPO_OWNER"), number)),
			},
		},
		Repo: &github.Repository{
			DefaultBranch: &runinfo.DefaultBranch,
			HTMLURL:       &runinfo.URL,
			Name:          &runinfo.Repository,
			Owner:         &github.User{Login: &runinfo.Owner},
		},
		Sender: &github.User{
			Login: &runinfo.Sender,
		},
	}

	err = payload.Send(ctx,
		cs,
		os.Getenv("TEST_EL_URL"),
		os.Getenv("TEST_EL_WEBHOOK_SECRET"),
		os.Getenv("TEST_GITHUB_API_URL"),
		os.Getenv("TEST_GITHUB_REPO_INSTALLATION_ID"),
		event,
		"issue_comment",
	)
	assert.NilError(t, err)

	cs.Log.Infof("Wait for the second repository update to be updated")
	err = twait.UntilRepositoryUpdated(ctx, cs.PipelineAsCode, targetNS, targetNS, 1, defaultTimeout)
	assert.NilError(t, err)

	cs.Log.Infof("Check if we have the repository set as succeeded")
	repo, err := cs.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(targetNS).Get(ctx, targetNS, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Assert(t, repo.Status[len(repo.Status)-1].Conditions[0].Status == corev1.ConditionTrue)
}
