//go:build e2e
// +build e2e

package test

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	ghlib "github.com/google/go-github/v39/github"
	pacv1alpha1 "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/github"
	tgithub "github.com/openshift-pipelines/pipelines-as-code/test/pkg/github"
	trepo "github.com/openshift-pipelines/pipelines-as-code/test/pkg/repository"
	twait "github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	"github.com/tektoncd/pipeline/pkg/names"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGithubPullRequest(t *testing.T) {
	for _, onWebhook := range []bool{false, true} {
		targetNS := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-ns")
		ctx := context.Background()
		if onWebhook && os.Getenv("TEST_HUB_REPO_OWNER_WEBHOOK") == "" {
			t.Skip("TEST_HUB_REPO_OWNER_WEBHOOK is not set")
			continue
		}
		runcnx, opts, ghprovider, err := githubSetup(ctx, onWebhook)
		assert.NilError(t, err)
		if onWebhook {
			runcnx.Clients.Log.Info("Testing with Direct Webhook integration")
		} else {
			runcnx.Clients.Log.Info("Testing with Github APPS integration")
		}

		repoinfo, err := createGithubRepoCRD(ctx, t, ghprovider, runcnx, opts, targetNS)
		assert.NilError(t, err)

		entries, err := getEntries("testdata/pipelinerun.yaml", targetNS, mainBranch, pullRequestEvent)
		assert.NilError(t, err)

		targetRefName := fmt.Sprintf("refs/heads/%s",
			names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-test"))

		title := "TestPullRequest "
		if onWebhook {
			title += "OnWebhook"
		}
		title += "- " + targetRefName

		sha, err := tgithub.PushFilesToRef(ctx, ghprovider.Client, title, repoinfo.GetDefaultBranch(), targetRefName,
			opts.Owner, opts.Repo, entries)
		assert.NilError(t, err)
		runcnx.Clients.Log.Infof("Commit %s has been created and pushed to %s", sha, targetRefName)

		number, err := tgithub.PRCreate(ctx, runcnx, ghprovider, opts.Owner, opts.Repo, targetRefName, repoinfo.GetDefaultBranch(), title)
		assert.NilError(t, err)

		defer ghtearDown(ctx, t, runcnx, ghprovider, number, targetRefName, targetNS, opts)

		checkSuccess(ctx, t, runcnx, opts, pullRequestEvent, targetNS, sha, title)
	}
}

func checkSuccess(ctx context.Context, t *testing.T, runcnx *params.Run, opts E2EOptions, onEvent, targetNS, sha, title string) {
	runcnx.Clients.Log.Infof("Waiting for Repository to be updated")
	waitOpts := twait.Opts{
		RepoName:        targetNS,
		Namespace:       targetNS,
		MinNumberStatus: 0,
		PollTimeout:     defaultTimeout,
		TargetSHA:       sha,
	}
	err := twait.UntilRepositoryUpdated(ctx, runcnx.Clients, waitOpts)
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

	pr, err := runcnx.Clients.Tekton.TektonV1beta1().PipelineRuns(targetNS).Get(ctx, laststatus.PipelineRunName, metav1.GetOptions{})
	assert.NilError(t, err)

	assert.Equal(t, onEvent, pr.Labels["pipelinesascode.tekton.dev/event-type"])
	assert.Equal(t, repo.GetName(), pr.Labels["pipelinesascode.tekton.dev/repository"])
	// assert.Equal(t, opts.Owner, pr.Labels["pipelinesascode.tekton.dev/sender"]) bitbucket is too weird for that
	assert.Equal(t, sha, pr.Labels["pipelinesascode.tekton.dev/sha"])
	assert.Equal(t, opts.Owner, pr.Labels["pipelinesascode.tekton.dev/url-org"])
	assert.Equal(t, opts.Repo, pr.Labels["pipelinesascode.tekton.dev/url-repository"])

	assert.Equal(t, sha, filepath.Base(pr.Annotations["pipelinesascode.tekton.dev/sha-url"]))
	assert.Equal(t, title, pr.Annotations["pipelinesascode.tekton.dev/sha-title"])
}

func createGithubRepoCRD(ctx context.Context, t *testing.T, ghprovider github.Provider, run *params.Run, opts E2EOptions, targetNS string) (*ghlib.Repository, error) {
	repoinfo, resp, err := ghprovider.Client.Repositories.Get(ctx, opts.Owner, opts.Repo)
	assert.NilError(t, err)

	if resp != nil && resp.Response.StatusCode == http.StatusNotFound {
		t.Errorf("Repository %s not found in %s", opts.Owner, opts.Repo)
	}

	repository := &pacv1alpha1.Repository{
		ObjectMeta: metav1.ObjectMeta{
			Name: targetNS,
		},
		Spec: pacv1alpha1.RepositorySpec{
			URL: repoinfo.GetHTMLURL(),
		},
	}

	err = trepo.CreateNS(ctx, targetNS, run)
	assert.NilError(t, err)

	if opts.DirectWebhook {
		token, _ := os.LookupEnv("TEST_GITHUB_TOKEN")
		apiURL, _ := os.LookupEnv("TEST_GITHUB_API_URL")
		err := createSecret(ctx, run, map[string]string{"token": token}, targetNS, "webhook-token")
		assert.NilError(t, err)
		repository.Spec.GitProvider = &pacv1alpha1.GitProvider{
			URL:    apiURL,
			Secret: &pacv1alpha1.GitProviderSecret{Name: "webhook-token", Key: "token"},
		}
	}

	err = trepo.CreateRepo(ctx, targetNS, run, repository)
	assert.NilError(t, err)
	return repoinfo, err
}

func getEntries(yamlfile, targetNS, targetBranch, targetEvent string) (map[string]string, error) {
	prun, err := ioutil.ReadFile(yamlfile)
	if err != nil {
		return nil, err
	}

	return map[string]string{
		".tekton/pr.yaml": fmt.Sprintf(string(prun), targetNS, targetBranch, targetEvent),
	}, nil
}

// Local Variables:
// compile-command: "go test -tags=e2e -v -info TestGithubPullRequest$ ."
// End:
