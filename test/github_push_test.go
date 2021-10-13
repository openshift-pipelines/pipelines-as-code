//go:build e2e
// +build e2e

package test

import (
	"context"
	"fmt"
	"testing"

	tgithub "github.com/openshift-pipelines/pipelines-as-code/test/pkg/github"
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

		ctx := context.Background()
		runcnx, opts, gvcs, err := githubSetup(ctx, onWebhook)
		assert.NilError(t, err)

		if onWebhook {
			runcnx.Clients.Log.Info("Testing with Direct Webhook integration")
		} else {
			runcnx.Clients.Log.Info("Testing with Github APPS integration")
		}

		repoinfo, err := createRepoCRD(ctx, t, gvcs, runcnx, opts, targetNS, targetEvent, targetBranch, runcnx)
		assert.NilError(t, err)

		entries, err := getEntries("testdata/pipelinerun-on-push.yaml", targetNS, targetBranch, targetEvent)
		assert.NilError(t, err)

		title := "TestPush "
		if onWebhook {
			title += "OnWebhook"
		}
		title += "- " + targetBranch

		targetRefName := fmt.Sprintf("refs/heads/%s", targetBranch)
		sha, err := tgithub.PushFilesToRef(ctx, gvcs.Client, title, repoinfo.GetDefaultBranch(), targetRefName, opts.Owner, opts.Repo, entries)
		runcnx.Clients.Log.Infof("Commit %s has been created and pushed to %s", sha, targetRefName)
		assert.NilError(t, err)
		defer tearDown(ctx, t, runcnx, gvcs, -1, targetRefName, targetNS, opts)

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
		assert.Assert(t, repo.Status[len(repo.Status)-1].Conditions[0].Status == corev1.ConditionTrue)
	}
}
