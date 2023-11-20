//go:build e2e
// +build e2e

package test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/go-github/v56/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	tgithub "github.com/openshift-pipelines/pipelines-as-code/test/pkg/github"
	twait "github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGithubPullRequestRetest(t *testing.T) {
	if os.Getenv("NIGHTLY_E2E_TEST") != "true" {
		t.Skip("Skipping test since only enabled for nightly")
	}
	ctx := context.Background()
	runcnx, ghcnx, opts, targetNS, targetRefName, prNumber, sha := tgithub.RunPullRequest(ctx, t,
		"Github Retest comment", []string{"testdata/pipelinerun.yaml"}, false, false)
	defer tgithub.TearDown(ctx, t, runcnx, ghcnx, prNumber, targetRefName, targetNS, opts)

	runcnx.Clients.Log.Infof("Creating /retest in PullRequest")
	_, _, err := ghcnx.Client.Issues.CreateComment(ctx,
		opts.Organization,
		opts.Repo, prNumber,
		&github.IssueComment{Body: github.String("/retest")})
	assert.NilError(t, err)

	runcnx.Clients.Log.Infof("Wait for the second repository update to be updated")
	waitOpts := twait.Opts{
		RepoName:        targetNS,
		Namespace:       targetNS,
		MinNumberStatus: 1,
		PollTimeout:     twait.DefaultTimeout,
		TargetSHA:       sha,
	}
	err = twait.UntilRepositoryUpdated(ctx, runcnx.Clients, waitOpts)
	assert.NilError(t, err)

	runcnx.Clients.Log.Infof("Check if we have the repository set as succeeded")
	repo, err := runcnx.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(targetNS).Get(ctx, targetNS, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, repo.Status[len(repo.Status)-1].Conditions[0].Status, corev1.ConditionTrue)
}

// TestGithubPullRequestGitOpsComments tests GitOps comments /test, /retest and /cancel commands.
func TestGithubPullRequestGitOpsComments(t *testing.T) {
	ctx := context.Background()
	runcnx, ghcnx, opts, targetNS, targetRefName, prNumber, sha := tgithub.RunPullRequest(ctx, t,
		"Github Retest comment", []string{"testdata/pipelinerun.yaml", "testdata/pipelinerun-gitops.yaml"}, false, false)
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
			comment: "/retest pr-gitops-comment",
			prNum:   3,
		},
		{
			name:    "Test and Cancel PipelineRun",
			comment: "/cancel pr-gitops-comment",
			prNum:   4,
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
			if tt.comment == "/cancel pr-gitops-comment" {
				runcnx.Clients.Log.Info("/test pr-gitops-comment on Pull Request before canceling")
				_, _, err := ghcnx.Client.Issues.CreateComment(ctx,
					opts.Organization,
					opts.Repo, prNumber,
					&github.IssueComment{Body: github.String("/test pr-gitops-comment")})
				assert.NilError(t, err)
				err = twait.UntilPipelineRunCreated(ctx, runcnx.Clients, waitOpts)
				assert.NilError(t, err)
			}
			runcnx.Clients.Log.Infof("%s on Pull Request", tt.comment)
			_, _, err = ghcnx.Client.Issues.CreateComment(ctx,
				opts.Organization,
				opts.Repo, prNumber,
				&github.IssueComment{Body: github.String(tt.comment)})
			assert.NilError(t, err)

			runcnx.Clients.Log.Info("Waiting for Repository to be updated")
			err = twait.UntilRepositoryUpdated(ctx, runcnx.Clients, waitOpts)
			assert.NilError(t, err)

			runcnx.Clients.Log.Infof("Check if we have the repository set as succeeded")
			repo, err := runcnx.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(targetNS).Get(ctx, targetNS, metav1.GetOptions{})
			assert.NilError(t, err)
			if tt.comment == "/cancel pr-gitops-comment" {
				assert.Equal(t, repo.Status[len(repo.Status)-1].Conditions[0].Status, corev1.ConditionFalse)
			} else {
				assert.Equal(t, repo.Status[len(repo.Status)-1].Conditions[0].Status, corev1.ConditionTrue)
			}

			pruns, err = runcnx.Clients.Tekton.TektonV1().PipelineRuns(targetNS).List(ctx, metav1.ListOptions{
				LabelSelector: fmt.Sprintf("%s=%s", keys.SHA, sha),
			})
			assert.NilError(t, err)
			assert.Equal(t, len(pruns.Items), tt.prNum)
		})
	}
}
