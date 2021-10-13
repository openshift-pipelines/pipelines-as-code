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
	runcnx, opts, ghcnx, err := githubSetup(ctx, false)
	assert.NilError(t, err)
	maxKepRuns := 1

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
			URL:       repoinfo.GetHTMLURL(),
			EventType: pullRequestEvent,
			Branch:    mainBranch,
		},
	}
	err = trepo.CreateNS(ctx, targetNS, runcnx)
	assert.NilError(t, err)

	err = trepo.CreateRepo(ctx, targetNS, runcnx, repository)
	assert.NilError(t, err)

	for prRun := 1; prRun <= maxKepRuns+1; prRun++ {
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
    pipelinesascode.tekton.dev/max-keep-runs: "%d"
    pipelinesascode.tekton.dev/test-current-info: "%d"
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

		sha, err := tgithub.PushFilesToRef(ctx, ghcnx.Client, "TestMaxKeepRuns - "+targetRefName, repoinfo.GetDefaultBranch(), targetRefName, opts.Owner, opts.Repo, entries)
		assert.NilError(t, err)
		runcnx.Clients.Log.Infof("Commit %s has been created and pushed to %s", sha, targetRefName)

		title := "TestMaxKeepRuns - " + targetRefName
		number, err := tgithub.PRCreate(ctx, runcnx, ghcnx, opts.Owner, opts.Repo, targetRefName, repoinfo.GetDefaultBranch(), title)
		assert.NilError(t, err)

		defer ghtearDown(ctx, t, runcnx, ghcnx, number, targetRefName, targetNS, opts)

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
	}

	prs, err := runcnx.Clients.Tekton.TektonV1beta1().PipelineRuns(targetNS).List(ctx, metav1.ListOptions{})
	assert.NilError(t, err)
	assert.Equal(t, len(prs.Items), maxKepRuns)
}

// Local Variables:
// compile-command: "go test -tags=e2e -v -run TestMaxKeepRuns$ ."
// End:
