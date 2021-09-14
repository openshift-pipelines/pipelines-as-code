//go:build e2e
// +build e2e

package test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	pacv1alpha1 "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	tgithub "github.com/openshift-pipelines/pipelines-as-code/test/pkg/github"
	trepo "github.com/openshift-pipelines/pipelines-as-code/test/pkg/repository"
	twait "github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"

	"github.com/tektoncd/pipeline/pkg/names"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Create two prs make sure only one is kept at the end
func TestMaxKeepRuns(t *testing.T) {
	targetNS := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-ns")
	ctx := context.Background()
	cs, opts, err := setup()
	assert.NilError(t, err)
	maxKepRuns := 1

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

	for prRun := 1; prRun <= maxKepRuns+1; prRun++ {
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
    pipelinesascode.tekton.dev/max-keep-runs: "%d"
    pipelinesascode.tekton.dev/test-current-run: "%d"
spec:
  pipelineSpec:
    tasks:
      - name: task
        taskSpec:
          steps:
            - name: task
              image: gcr.io/google-containers/busybox
              command: ["/bin/echo", "HELLOMOTO"]
`, targetNS, mainBranch, pullRequestEvent, maxKepRuns, prRun),
		}

		targetRefName := fmt.Sprintf("refs/heads/%s",
			names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-test"))

		sha, err := tgithub.PushFilesToRef(ctx, cs.GithubClient.Client, "TestMaxKeepRuns - "+targetRefName, repoinfo.GetDefaultBranch(), targetRefName, opts.Owner, opts.Repo, entries)
		assert.NilError(t, err)
		cs.Log.Infof("Commit %s has been created and pushed to %s", sha, targetRefName)

		title := "TestMaxKeepRuns - " + targetRefName
		number, err := tgithub.PRCreate(ctx, cs, opts.Owner, opts.Repo, targetRefName, repoinfo.GetDefaultBranch(), title)
		assert.NilError(t, err)

		defer tearDown(ctx, t, cs, number, targetRefName, targetNS, opts)

		cs.Log.Infof("Waiting for Repository to be updated")
		err = twait.UntilRepositoryUpdated(ctx, cs.PipelineAsCode, targetNS, targetNS, 0, defaultTimeout)
		assert.NilError(t, err)
	}

	prs, err := cs.Tekton.TektonV1alpha1().PipelineRuns(targetNS).List(ctx, metav1.ListOptions{})
	assert.NilError(t, err)
	assert.Equal(t, len(prs.Items), maxKepRuns)
}

// Local Variables:
// compile-command: "go test -tags=e2e -v -run TestMaxKeepRuns$ ."
// End:
