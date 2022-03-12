//go:build e2e
// +build e2e

package test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"

	tgithub "github.com/openshift-pipelines/pipelines-as-code/test/pkg/github"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/payload"
	twait "github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	"github.com/tektoncd/pipeline/pkg/names"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGithubPush(t *testing.T) {
	for _, onWebhook := range []bool{false, true} {
		targetNS := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-push")
		targetBranch := targetNS
		targetEvent := "push"
		if onWebhook && os.Getenv("TEST_GITHUB_REPO_OWNER_WEBHOOK") == "" {
			t.Skip("TEST_GITHUB_REPO_OWNER_WEBHOOK is not set")
			continue
		}

		ctx := context.Background()
		runcnx, opts, gprovider, err := tgithub.Setup(ctx, onWebhook)
		assert.NilError(t, err)

		if onWebhook {
			runcnx.Clients.Log.Info("Testing with Direct Webhook integration")
		} else {
			runcnx.Clients.Log.Info("Testing with Github APPS integration")
		}
		repoinfo, resp, err := gprovider.Client.Repositories.Get(ctx, opts.Organization, opts.Repo)
		assert.NilError(t, err)
		if resp != nil && resp.Response.StatusCode == http.StatusNotFound {
			t.Errorf("Repository %s not found in %s", opts.Organization, opts.Repo)
		}
		err = tgithub.CreateCRD(ctx, t, repoinfo, runcnx, opts, targetNS)
		assert.NilError(t, err)

		entries, err := payload.GetEntries("testdata/pipelinerun-on-push.yaml", targetNS, targetBranch, targetEvent)
		assert.NilError(t, err)

		title := "TestPush "
		if onWebhook {
			title += "OnWebhook"
		}
		title += "- " + targetBranch

		targetRefName := fmt.Sprintf("refs/heads/%s", targetBranch)
		sha, err := tgithub.PushFilesToRef(ctx, gprovider.Client, title, repoinfo.GetDefaultBranch(), targetRefName, opts.Organization, opts.Repo, entries)
		runcnx.Clients.Log.Infof("Commit %s has been created and pushed to %s", sha, targetRefName)
		assert.NilError(t, err)
		defer tgithub.TearDown(ctx, t, runcnx, gprovider, -1, targetRefName, targetNS, opts)

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
		assert.Assert(t, repo.Status[len(repo.Status)-1].Conditions[0].Status == corev1.ConditionTrue)
	}
}
