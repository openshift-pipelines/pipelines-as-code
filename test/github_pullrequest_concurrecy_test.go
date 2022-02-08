//go:build e2e
// +build e2e

package test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	ghlib "github.com/google/go-github/v39/github"
	pacv1alpha1 "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/github"
	tgithub "github.com/openshift-pipelines/pipelines-as-code/test/pkg/github"
	trepo "github.com/openshift-pipelines/pipelines-as-code/test/pkg/repository"
	"github.com/tektoncd/pipeline/pkg/names"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGithubPullRequestWithConcurrency(t *testing.T) {
	for _, onWebhook := range []bool{true, false} {
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

		// create repo with concurrency defined
		repoinfo, err := createGithubRepoCRDC(ctx, t, ghprovider, runcnx, 1, opts, targetNS)
		assert.NilError(t, err)

		// create pr from 2 branches to main
		// pipelinerun for pr which is created 2nd must be started
		// after completion of first

		entries, err := getEntries("testdata/pipelinerun_long_duration.yaml", targetNS, mainBranch, pullRequestEvent)
		assert.NilError(t, err)

		// branch 1
		targetRefName := fmt.Sprintf("refs/heads/%s",
			names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-test"))

		// branch 2
		targetRefName2 := fmt.Sprintf("refs/heads/%s",
			names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-test"))

		title := "TestPullRequest "
		if onWebhook {
			title += "OnWebhook"
		}

		title2 := title
		title2 += "- " + targetRefName2

		title += "- " + targetRefName

		sha, err := tgithub.PushFilesToRef(ctx, ghprovider.Client, title, repoinfo.GetDefaultBranch(), targetRefName,
			opts.Organization, opts.Repo, entries)
		assert.NilError(t, err)
		runcnx.Clients.Log.Infof("Commit %s has been created and pushed to %s", sha, targetRefName)

		sha2, err := tgithub.PushFilesToRef(ctx, ghprovider.Client, title2, repoinfo.GetDefaultBranch(), targetRefName2,
			opts.Organization, opts.Repo, entries)
		assert.NilError(t, err)
		runcnx.Clients.Log.Infof("Commit %s has been created and pushed to %s", sha2, targetRefName)

		// create first pr
		number, err := tgithub.PRCreate(ctx, runcnx, ghprovider, opts.Organization, opts.Repo, targetRefName, repoinfo.GetDefaultBranch(), title)
		assert.NilError(t, err)

		// create second pr
		number2, err := tgithub.PRCreate(ctx, runcnx, ghprovider, opts.Organization, opts.Repo, targetRefName2, repoinfo.GetDefaultBranch(), title2)
		assert.NilError(t, err)

		// wait for a few second before fetching pipelineRun
		runcnx.Clients.Log.Info("waiting for some time after creating pull request")
		time.Sleep(10 * time.Second)

		allPR, err := runcnx.Clients.Tekton.TektonV1beta1().PipelineRuns(targetNS).List(ctx, metav1.ListOptions{})
		assert.NilError(t, err)

		// there must be 2 pipelineRun created
		assert.Equal(t, len(allPR.Items), 2)

		// at a time only one pipelineRun must be running as concurrecy limit is 1,
		// and rest of them would be in pending state

		running := false
		runningPRSha := ""
		for _, pr := range allPR.Items {
			// if spec.Status is nil that means the pipelineRun is in running state
			if pr.Spec.Status == "" {
				if running {
					// there is already pipelineRun running
					runcnx.Clients.Log.Fatal("more than one pipelineRun in running state")
				}
				runcnx.Clients.Log.Infof("pipelineRun %s is in running state", pr.Name)
				running = true
				runningPRSha = pr.Labels["pipelinesascode.tekton.dev/sha"]
			} else {
				runcnx.Clients.Log.Infof("pipelineRun %s is in %s state", pr.Name, pr.Spec.Status)
			}
		}

		defer ghtearDown(ctx, t, runcnx, ghprovider, number2, targetRefName2, targetNS, opts)
		defer ghtearDown(ctx, t, runcnx, ghprovider, number, targetRefName, targetNS, opts)

		checkSuccess(ctx, t, runcnx, opts, pullRequestEvent, targetNS, runningPRSha, title)
	}
}

func createGithubRepoCRDC(ctx context.Context, t *testing.T, ghprovider github.Provider, run *params.Run, concurrency int, opts E2EOptions, targetNS string) (*ghlib.Repository, error) {
	repoinfo, resp, err := ghprovider.Client.Repositories.Get(ctx, opts.Organization, opts.Repo)
	assert.NilError(t, err)

	if resp != nil && resp.Response.StatusCode == http.StatusNotFound {
		t.Errorf("Repository %s not found in %s", opts.Organization, opts.Repo)
	}

	repository := &pacv1alpha1.Repository{
		ObjectMeta: metav1.ObjectMeta{
			Name: targetNS,
		},
		Spec: pacv1alpha1.RepositorySpec{
			URL:              repoinfo.GetHTMLURL(),
			ConcurrencyLimit: concurrency,
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

// Local Variables:
// compile-command: "go test -tags=e2e -v -info TestGithubPullRequest$ ."
// End:
