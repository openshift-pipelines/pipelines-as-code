package repository

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v42/github"
	"github.com/jonboulle/clockwork"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	tcli "github.com/openshift-pipelines/pipelines-as-code/pkg/test/cli"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/golden"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativeapis "knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck/v1beta1"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestRoot(t *testing.T) {
	buf := &bytes.Buffer{}
	s, err := tcli.ExecuteCommand(Root(&params.Run{}, &cli.IOStreams{Out: buf, ErrOut: buf}), "help")
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(s, "repository, repo, repositories"))
}

func TestCommands(t *testing.T) {
	tests := []struct {
		name    string
		command func(c *params.Run, ioStreams *cli.IOStreams) *cobra.Command
		want    *cobra.Command
	}{
		{
			name:    "List",
			command: ListCommand,
		},
		{
			name:    "Describe",
			command: DescribeCommand,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cw := clockwork.NewFakeClock()
			const (
				nsName   = "ns"
				repoName = "repo1"
			)
			statuses := []v1alpha1.RepositoryRunStatus{
				{
					Status: v1beta1.Status{
						Conditions: []knativeapis.Condition{
							{
								Reason: "Success",
							},
						},
					},
					PipelineRunName: "pipelinerun1",
					StartTime:       &metav1.Time{Time: cw.Now().Add(-16 * time.Minute)},
					CompletionTime:  &metav1.Time{Time: cw.Now().Add(-15 * time.Minute)},
					SHA:             github.String("SHA"),
					SHAURL:          github.String("https://anurl.com/repo/owner/commit/SHA"),
					Title:           github.String("A title"),
					EventType:       github.String("pull_request"),
					TargetBranch:    github.String("TargetBranch"),
					LogURL:          github.String("https://everywhere.anwywhere"),
				},
			}
			repositories := []*v1alpha1.Repository{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      repoName,
						Namespace: nsName,
					},
					Spec: v1alpha1.RepositorySpec{
						URL: "https://anurl.com/repo/owner",
					},
					Status: statuses,
				},
			}
			tdata := testclient.Data{
				Namespaces: []*corev1.Namespace{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: nsName,
						},
					},
				},
				Repositories: repositories,
			}
			ctx, _ := rtesting.SetupFakeContext(t)
			stdata, _ := testclient.SeedTestData(t, ctx, tdata)
			cs := &params.Run{
				Clients: clients.Clients{
					ClientInitialized: true,
					PipelineAsCode:    stdata.PipelineAsCode,
					Tekton:            stdata.Pipeline,
				},
			}
			buf := new(bytes.Buffer)
			ioStream := &cli.IOStreams{Out: buf, ErrOut: buf}
			cmd := tt.command(cs, ioStream)
			cmd.SetOut(buf)
			_, err := tcli.ExecuteCommand(cmd, "-n", nsName, repoName)
			assert.NilError(t, err)

			golden.Assert(t, buf.String(), strings.ReplaceAll(fmt.Sprintf("%s.golden", t.Name()), "/", "-"))
		})
	}
}
