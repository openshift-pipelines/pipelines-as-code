//go:build e2e
// +build e2e

package test

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pacv1alpha1 "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/configmap"
	tgithub "github.com/openshift-pipelines/pipelines-as-code/test/pkg/github"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/payload"
	trepo "github.com/openshift-pipelines/pipelines-as-code/test/pkg/repository"
	twait "github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	"github.com/tektoncd/pipeline/pkg/names"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGithubPullRequestScopeTokenToListOfRepos(t *testing.T) {
	// if os.Getenv("NIGHTLY_E2E_TEST") != "true" {
	//	 t.Skip("Skipping test since only enabled for nightly")
	// }
	targetNS := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-ns")
	ctx := context.Background()
	runcnx, opts, ghcnx, err := tgithub.Setup(ctx, false)
	assert.NilError(t, err)

	var remoteTaskURL, remoteTaskName string
	if os.Getenv("TEST_GITHUB_PRIVATE_TASK_URL") != "" {
		remoteTaskURL = os.Getenv("TEST_GITHUB_PRIVATE_TASK_URL")
	} else {
		t.Error("Env TEST_GITHUB_PRIVATE_TASK_URL not provided")
		return
	}

	if os.Getenv("TEST_GITHUB_PRIVATE_TASK_NAME") != "" {
		remoteTaskName = os.Getenv("TEST_GITHUB_PRIVATE_TASK_NAME")
	} else {
		t.Error("Env TEST_GITHUB_PRIVATE_TASK_NAME not provided")
		return
	}

	data := map[string]string{"secret-github-app-token-scoped": "false"}
	defer configmap.ChangeGlobalConfig(ctx, t, runcnx, data)()

	entries, err := payload.GetEntries(map[string]string{
		".tekton/pr.yaml":                              "testdata/pipelinerun_remote_task_annotations.yaml",
		".tekton/pipeline.yaml":                        "testdata/pipeline_in_tektondir.yaml",
		".other-tasks/task-referenced-internally.yaml": "testdata/task_referenced_internally.yaml",
	}, targetNS, options.MainBranch, options.PullRequestEvent, map[string]string{
		"RemoteTaskURL":  remoteTaskURL,
		"RemoteTaskName": remoteTaskName,
	})
	assert.NilError(t, err)

	repoinfo, resp, err := ghcnx.Client.Repositories.Get(ctx, opts.Organization, opts.Repo)
	assert.NilError(t, err)
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		t.Errorf("Repository %s not found in %s", opts.Organization, opts.Repo)
	}

	splittedValue := []string{}
	if remoteTaskURL != "" {
		urlData, err := url.ParseRequestURI(remoteTaskURL)
		assert.NilError(t, err)
		splittedValue = strings.Split(urlData.Path, "/")
	}
	repository := &pacv1alpha1.Repository{
		ObjectMeta: metav1.ObjectMeta{
			Name: targetNS,
		},
		Spec: pacv1alpha1.RepositorySpec{
			URL: repoinfo.GetHTMLURL(),
			Settings: &pacv1alpha1.Settings{
				GithubAppTokenScopeRepos: []string{splittedValue[1] + "/" + splittedValue[2]},
			},
		},
	}

	err = trepo.CreateNS(ctx, targetNS, runcnx)
	assert.NilError(t, err)

	err = trepo.CreateRepo(ctx, targetNS, runcnx, repository)
	assert.NilError(t, err)

	repositoryForPrivateRepo := &pacv1alpha1.Repository{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: targetNS,
		},
		Spec: pacv1alpha1.RepositorySpec{
			URL: "https://github.com/" + splittedValue[1] + "/" + splittedValue[2],
		},
	}

	err = trepo.CreateRepo(ctx, targetNS, runcnx, repositoryForPrivateRepo)
	assert.NilError(t, err)

	targetRefName := fmt.Sprintf("refs/heads/%s",
		names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-test"))

	sha, err := tgithub.PushFilesToRef(ctx, ghcnx.Client, "TestPullRequestRemoteAnnotations - "+targetRefName, repoinfo.GetDefaultBranch(), targetRefName, opts.Organization, opts.Repo, entries)
	assert.NilError(t, err)
	runcnx.Clients.Log.Infof("Commit %s has been created and pushed to %s", sha, targetRefName)

	title := "TestPullRequestRemoteAnnotations - " + targetRefName
	number, err := tgithub.PRCreate(ctx, runcnx, ghcnx, opts.Organization, opts.Repo, targetRefName, repoinfo.GetDefaultBranch(), title)
	assert.NilError(t, err)

	defer tgithub.TearDown(ctx, t, runcnx, ghcnx, number, targetRefName, targetNS, opts)

	runcnx.Clients.Log.Infof("Waiting for Repository to be updated")
	waitOpts := twait.Opts{
		RepoName:        targetNS,
		Namespace:       targetNS,
		MinNumberStatus: 0,
		PollTimeout:     twait.DefaultTimeout,
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

	pr, err := runcnx.Clients.Tekton.TektonV1().PipelineRuns(targetNS).Get(ctx, laststatus.PipelineRunName, metav1.GetOptions{})
	assert.NilError(t, err)

	assert.Equal(t, options.PullRequestEvent, pr.Labels["pipelinesascode.tekton.dev/event-type"])
	assert.Equal(t, repo.GetName(), pr.Labels["pipelinesascode.tekton.dev/repository"])
	assert.Equal(t, sha, pr.Labels["pipelinesascode.tekton.dev/sha"])
	assert.Equal(t, opts.Organization, pr.Labels["pipelinesascode.tekton.dev/url-org"])
	assert.Equal(t, opts.Repo, pr.Labels["pipelinesascode.tekton.dev/url-repository"])

	assert.Equal(t, sha, filepath.Base(pr.Annotations["pipelinesascode.tekton.dev/sha-url"]))
	assert.Equal(t, title, pr.Annotations["pipelinesascode.tekton.dev/sha-title"])
}

// Local Variables:
// compile-command: "go test -tags=e2e -v -run ^TestGithubPullRequestScopeTokenToListOfRepos$"
// End:
