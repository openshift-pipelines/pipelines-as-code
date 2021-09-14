//go:build e2e
// +build e2e

package test

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	pacv1alpha1 "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	tgithub "github.com/openshift-pipelines/pipelines-as-code/test/pkg/github"
	trepo "github.com/openshift-pipelines/pipelines-as-code/test/pkg/repository"
	twait "github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	"github.com/tektoncd/pipeline/pkg/names"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPullRequest(t *testing.T) {
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

	sha, err := tgithub.PushFilesToRef(ctx, cs.GithubClient.Client, "TestPullRequest - "+targetRefName, repoinfo.GetDefaultBranch(), targetRefName, opts.Owner, opts.Repo, entries)
	assert.NilError(t, err)
	cs.Log.Infof("Commit %s has been created and pushed to %s", sha, targetRefName)

	title := "TestPullRequest - " + targetRefName
	number, err := tgithub.PRCreate(ctx, cs, opts.Owner, opts.Repo, targetRefName, repoinfo.GetDefaultBranch(), title)
	assert.NilError(t, err)

	defer tearDown(ctx, t, cs, number, targetRefName, targetNS, opts)

	cs.Log.Infof("Waiting for Repository to be updated")
	err = twait.UntilRepositoryUpdated(ctx, cs.PipelineAsCode, targetNS, targetNS, 0, defaultTimeout)
	assert.NilError(t, err)

	cs.Log.Infof("Check if we have the repository set as succeeded")
	repo, err := cs.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(targetNS).Get(ctx, targetNS, metav1.GetOptions{})
	assert.NilError(t, err)
	laststatus := repo.Status[len(repo.Status)-1]
	assert.Equal(t, corev1.ConditionTrue, laststatus.Conditions[0].Status)
	assert.Equal(t, sha, *laststatus.SHA)
	assert.Equal(t, sha, filepath.Base(*laststatus.SHAURL))
	assert.Equal(t, title, *laststatus.Title)
	assert.Assert(t, *laststatus.LogURL != "")

	pr, err := cs.Tekton.TektonV1alpha1().PipelineRuns(targetNS).Get(ctx, laststatus.PipelineRunName, metav1.GetOptions{})
	assert.NilError(t, err)

	assert.Equal(t, "pull_request", pr.Labels["pipelinesascode.tekton.dev/event-type"])
	assert.Equal(t, repo.GetName(), pr.Labels["pipelinesascode.tekton.dev/repository"])
	assert.Equal(t, opts.Owner, pr.Labels["pipelinesascode.tekton.dev/sender"])
	assert.Equal(t, sha, pr.Labels["pipelinesascode.tekton.dev/sha"])
	assert.Equal(t, opts.Owner, pr.Labels["pipelinesascode.tekton.dev/url-org"])
	assert.Equal(t, opts.Repo, pr.Labels["pipelinesascode.tekton.dev/url-repository"])

	assert.Equal(t, sha, filepath.Base(pr.Annotations["pipelinesascode.tekton.dev/sha-url"]))
	assert.Equal(t, title, pr.Annotations["pipelinesascode.tekton.dev/sha-title"])
}

// Local Variables:
// compile-command: "go test -tags=e2e -v -run TestPullRequest$ ."
// End:
