//go:build e2e
// +build e2e

package test

import (
	"context"
	"testing"

	"github.com/google/go-github/v56/github"
	tgithub "github.com/openshift-pipelines/pipelines-as-code/test/pkg/github"
	twait "github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGithubSecondPushRequestGitOpsComments(t *testing.T) {
	ctx := context.Background()
	runcnx, ghcnx, opts, targetNS, targetRefName, prNumber, sha := tgithub.RunPushRequest(ctx, t,
		"Github Push Request", []string{"testdata/pipelinerun-on-push.yaml", "testdata/pipelinerun.yaml"}, true, false)
	defer tgithub.TearDown(ctx, t, runcnx, ghcnx, prNumber, targetRefName, targetNS, opts)

	pruns, err := runcnx.Clients.Tekton.TektonV1().PipelineRuns(targetNS).List(ctx, metav1.ListOptions{})
	assert.NilError(t, err)
	assert.Equal(t, len(pruns.Items), 2)

	tests := []struct {
		name, comment string
		prNum         int
	}{
		{
			name:    "Retest",
			comment: "/retest branch:" + targetNS,
			prNum:   4,
		},
		{
			name:    "Test and Cancel PipelineRun",
			comment: "/cancel pipelinerun-on-push branch:" + targetNS,
			prNum:   5,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			waitOpts := twait.Opts{
				RepoName:        targetNS,
				Namespace:       targetNS,
				MinNumberStatus: tt.prNum,
				PollTimeout:     twait.DefaultTimeout,
				TargetSHA:       sha,
			}
			if tt.comment == "/cancel pipelinerun-on-push branch:"+targetNS {
				runcnx.Clients.Log.Info("/test pipelinerun-on-push on Push Request before canceling")
				_, _, err = ghcnx.Client.Repositories.CreateComment(ctx,
					opts.Organization,
					opts.Repo, sha,
					&github.RepositoryComment{Body: github.String("/test pipelinerun-on-push branch:" + targetNS)})
				assert.NilError(t, err)
				err = twait.UntilPipelineRunCreated(ctx, runcnx.Clients, waitOpts)
				assert.NilError(t, err)
			}
			runcnx.Clients.Log.Infof("%s on Push Request", tt.comment)
			_, _, err = ghcnx.Client.Repositories.CreateComment(ctx,
				opts.Organization,
				opts.Repo, sha,
				&github.RepositoryComment{Body: github.String(tt.comment)})
			assert.NilError(t, err)

			runcnx.Clients.Log.Info("Waiting for Repository to be updated")
			err = twait.UntilRepositoryUpdated(ctx, runcnx.Clients, waitOpts)
			assert.NilError(t, err)

			runcnx.Clients.Log.Infof("Check if we have the repository set as succeeded")
			repo, err := runcnx.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(targetNS).Get(ctx, targetNS, metav1.GetOptions{})
			assert.NilError(t, err)
			if tt.comment == "/cancel pipelinerun-on-push branch:"+targetNS {
				assert.Equal(t, repo.Status[len(repo.Status)-1].Conditions[0].Status, corev1.ConditionFalse)
			} else {
				assert.Equal(t, repo.Status[len(repo.Status)-1].Conditions[0].Status, corev1.ConditionTrue)
			}

			pruns, err = runcnx.Clients.Tekton.TektonV1().PipelineRuns(targetNS).List(ctx, metav1.ListOptions{})
			assert.NilError(t, err)
			assert.Equal(t, len(pruns.Items), tt.prNum)
		})
	}
}
