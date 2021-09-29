//go:build e2e
// +build e2e

package test

import (
	"context"
	"fmt"
	"io/ioutil"
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

func TestPullRequestRemoteAnnotations(t *testing.T) {
	targetNS := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-ns")
	ctx := context.Background()
	runcnx, opts, ghcnx, err := setup(ctx, false)
	assert.NilError(t, err)

	prun, err := ioutil.ReadFile("testdata/pipelinerun_remote_annotations.yaml")
	assert.NilError(t, err)

	pipeline, err := ioutil.ReadFile("testdata/pipeline_remote_annotations.yaml")
	assert.NilError(t, err)

	taskreferencedinternally, err := ioutil.ReadFile("testdata/task_referenced_internally.yaml")
	assert.NilError(t, err)

	entries := map[string]string{
		".tekton/pr.yaml": fmt.Sprintf(string(prun),
			targetNS, mainBranch, pullRequestEvent),
		".tekton/pipeline.yaml":                        string(pipeline),
		".other-tasks/task-referenced-internally.yaml": string(taskreferencedinternally),
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

	err = trepo.CreateNS(ctx, targetNS, runcnx)
	assert.NilError(t, err)

	err = trepo.CreateRepo(ctx, targetNS, runcnx, repository)
	assert.NilError(t, err)

	targetRefName := fmt.Sprintf("refs/heads/%s",
		names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-test"))

	sha, err := tgithub.PushFilesToRef(ctx, ghcnx.Client, "TestPullRequestRemoteAnnotations - "+targetRefName, repoinfo.GetDefaultBranch(), targetRefName, opts.Owner, opts.Repo, entries)
	assert.NilError(t, err)
	runcnx.Clients.Log.Infof("Commit %s has been created and pushed to %s", sha, targetRefName)

	title := "TestPullRequestRemoteAnnotations - " + targetRefName
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

	runcnx.Clients.Log.Infof("Check if we have the repository set as succeeded")
	repo, err := runcnx.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(targetNS).Get(ctx, targetNS, metav1.GetOptions{})
	assert.NilError(t, err)
	laststatus := repo.Status[len(repo.Status)-1]
	assert.Equal(t, corev1.ConditionTrue, laststatus.Conditions[0].Status)
	assert.Equal(t, sha, *laststatus.SHA)
	assert.Equal(t, sha, filepath.Base(*laststatus.SHAURL))
	assert.Equal(t, title, *laststatus.Title)
	assert.Assert(t, *laststatus.LogURL != "")

	pr, err := runcnx.Clients.Tekton.TektonV1alpha1().PipelineRuns(targetNS).Get(ctx, laststatus.PipelineRunName, metav1.GetOptions{})
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
// compile-command: "go test -tags=e2e -v -info TestPullRequestRemoteAnnotations$ ."
// End:
