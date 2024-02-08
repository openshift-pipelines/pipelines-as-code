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
	g := &tgithub.PRTest{
		Label:            "Github push request",
		YamlFiles:        []string{"testdata/pipelinerun-on-push.yaml", "testdata/pipelinerun.yaml"},
		SecondController: false,
	}
	g.RunPushRequest(ctx, t)
	defer g.TearDown(ctx, t)

	pruns, err := g.Cnx.Clients.Tekton.TektonV1().PipelineRuns(g.TargetNamespace).List(ctx, metav1.ListOptions{})
	assert.NilError(t, err)
	assert.Equal(t, len(pruns.Items), 2)

	tests := []struct {
		name, comment string
		prNum         int
	}{
		{
			name:    "Retest",
			comment: "/retest branch:" + g.TargetNamespace,
			prNum:   4,
		},
		{
			name:    "Test and Cancel PipelineRun",
			comment: "/cancel pipelinerun-on-push branch:" + g.TargetNamespace,
			prNum:   5,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			waitOpts := twait.Opts{
				RepoName:        g.TargetNamespace,
				Namespace:       g.TargetNamespace,
				MinNumberStatus: tt.prNum,
				PollTimeout:     twait.DefaultTimeout,
				TargetSHA:       g.SHA,
			}
			if tt.comment == "/cancel pipelinerun-on-push branch:"+g.TargetNamespace {
				g.Cnx.Clients.Log.Info("/test pipelinerun-on-push on Push Request before canceling")
				_, _, err = g.Provider.Client.Repositories.CreateComment(ctx,
					g.Options.Organization,
					g.Options.Repo, g.SHA,
					&github.RepositoryComment{Body: github.String("/test pipelinerun-on-push branch:" + g.TargetNamespace)})
				assert.NilError(t, err)
				err = twait.UntilPipelineRunCreated(ctx, g.Cnx.Clients, waitOpts)
				assert.NilError(t, err)
			}
			g.Cnx.Clients.Log.Infof("%s on Push Request", tt.comment)
			_, _, err = g.Provider.Client.Repositories.CreateComment(ctx,
				g.Options.Organization,
				g.Options.Repo, g.SHA,
				&github.RepositoryComment{Body: github.String(tt.comment)})
			assert.NilError(t, err)

			g.Cnx.Clients.Log.Info("Waiting for Repository to be updated")
			_, err = twait.UntilRepositoryUpdated(ctx, g.Cnx.Clients, waitOpts)
			assert.NilError(t, err)

			g.Cnx.Clients.Log.Infof("Check if we have the repository set as succeeded")
			repo, err := g.Cnx.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(g.TargetNamespace).Get(ctx, g.TargetNamespace, metav1.GetOptions{})
			assert.NilError(t, err)
			if tt.comment == "/cancel pipelinerun-on-push branch:"+g.TargetNamespace {
				assert.Equal(t, repo.Status[len(repo.Status)-1].Conditions[0].Status, corev1.ConditionFalse)
			} else {
				assert.Equal(t, repo.Status[len(repo.Status)-1].Conditions[0].Status, corev1.ConditionTrue)
			}

			pruns, err = g.Cnx.Clients.Tekton.TektonV1().PipelineRuns(g.TargetNamespace).List(ctx, metav1.ListOptions{})
			assert.NilError(t, err)
			assert.Equal(t, len(pruns.Items), tt.prNum)
		})
	}
}
