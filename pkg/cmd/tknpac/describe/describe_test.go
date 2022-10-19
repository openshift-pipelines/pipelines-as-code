package describe

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v47/github"
	"github.com/jonboulle/clockwork"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/consoleui"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	tcli "github.com/openshift-pipelines/pipelines-as-code/pkg/test/cli"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	tektontest "github.com/openshift-pipelines/pipelines-as-code/pkg/test/tekton"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"gotest.tools/v3/golden"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativeapis "knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck/v1beta1"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestDescribe(t *testing.T) {
	cw := clockwork.NewFakeClock()
	ns := "ns"
	running := tektonv1beta1.PipelineRunReasonRunning.String()
	type args struct {
		currentNamespace string
		repoName         string
		statuses         []v1alpha1.RepositoryRunStatus
		opts             *cli.PacCliOpts
		pruns            []*tektonv1beta1.PipelineRun
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "live run and repository run",
			args: args{
				repoName:         "test-run",
				currentNamespace: ns,
				opts:             &cli.PacCliOpts{},
				pruns: []*tektonv1beta1.PipelineRun{
					tektontest.MakePRCompletion(cw, "running", ns, running,
						map[string]string{
							"pipelinesascode.tekton.dev/repository": "test-run",
							"pipelinesascode.tekton.dev/branch":     "tartanpion",
							"pipelinesascode.tekton.dev/event-type": "papayolo",
						}, 30),
				},
				statuses: []v1alpha1.RepositoryRunStatus{
					{
						CollectedTaskInfos: &map[string]v1alpha1.TaskInfos{},
						Status: v1beta1.Status{
							Conditions: []knativeapis.Condition{
								{
									Reason: "Success",
								},
							},
						},
						PipelineRunName: "pipelinerun1",
						LogURL:          github.String("https://everywhere.anwywhere"),
						StartTime:       &metav1.Time{Time: cw.Now().Add(-16 * time.Minute)},
						CompletionTime:  &metav1.Time{Time: cw.Now().Add(-15 * time.Minute)},
						SHA:             github.String("SHA"),
						SHAURL:          github.String("https://anurl.com/commit/SHA"),
						Title:           github.String("A title"),
						TargetBranch:    github.String("TargetBranch"),
						EventType:       github.String("propseryouplaboun"),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "one live run",
			args: args{
				repoName:         "test-run",
				currentNamespace: ns,
				opts:             &cli.PacCliOpts{},
				pruns: []*tektonv1beta1.PipelineRun{
					tektontest.MakePRCompletion(cw, "running", ns, running,
						map[string]string{
							"pipelinesascode.tekton.dev/repository": "test-run",
							"pipelinesascode.tekton.dev/branch":     "tartanpion",
						}, 30),
				},
				statuses: []v1alpha1.RepositoryRunStatus{},
			},
			wantErr: false,
		},
		{
			name: "multiple live runs",
			args: args{
				repoName:         "test-run",
				currentNamespace: ns,
				opts:             &cli.PacCliOpts{},
				pruns: []*tektonv1beta1.PipelineRun{
					tektontest.MakePRCompletion(cw, "running", ns, running,
						map[string]string{
							"pipelinesascode.tekton.dev/repository": "test-run",
							"pipelinesascode.tekton.dev/branch":     "tartanpion",
						}, 30),
					tektontest.MakePRCompletion(cw, "running2", ns, running,
						map[string]string{
							"pipelinesascode.tekton.dev/repository": "test-run",
							"pipelinesascode.tekton.dev/branch":     "vavaroom",
						}, 30),
				},
				statuses: []v1alpha1.RepositoryRunStatus{},
			},
			wantErr: false,
		},
		{
			name: "collect failures",
			args: args{
				repoName:         "test-run",
				currentNamespace: "namespace",
				opts: &cli.PacCliOpts{
					Namespace: "optnamespace",
				},
				statuses: []v1alpha1.RepositoryRunStatus{
					{
						Status: v1beta1.Status{
							Conditions: []knativeapis.Condition{
								{
									Reason: "Success",
								},
							},
						},
						CollectedTaskInfos: &map[string]v1alpha1.TaskInfos{
							"task1": {
								Reason:     tektonv1beta1.PipelineRunReasonFailed.String(),
								LogSnippet: "Error error miss robinson",
							},
						},
						PipelineRunName: "pipelinerun1",
						LogURL:          github.String("https://everywhere.anwywhere"),
						StartTime:       &metav1.Time{Time: cw.Now().Add(-16 * time.Minute)},
						CompletionTime:  &metav1.Time{Time: cw.Now().Add(-15 * time.Minute)},
						SHA:             github.String("SHA"),
						SHAURL:          github.String("https://anurl.com/commit/SHA"),
						Title:           github.String("A title"),
						TargetBranch:    github.String("TargetBranch"),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "use real time",
			args: args{
				repoName:         "test-run",
				currentNamespace: "namespace",
				opts: &cli.PacCliOpts{
					Namespace:   "optnamespace",
					UseRealTime: true,
				},
				statuses: []v1alpha1.RepositoryRunStatus{
					{
						Status: v1beta1.Status{
							Conditions: []knativeapis.Condition{
								{
									Reason: "Success",
								},
							},
						},
						CollectedTaskInfos: &map[string]v1alpha1.TaskInfos{},
						PipelineRunName:    "pipelinerun1",
						LogURL:             github.String("https://everywhere.anwywhere"),
						StartTime:          &metav1.Time{Time: cw.Now().Add(-16 * time.Minute)},
						CompletionTime:     &metav1.Time{Time: cw.Now().Add(-15 * time.Minute)},
						SHA:                github.String("SHA"),
						SHAURL:             github.String("https://anurl.com/commit/SHA"),
						Title:              github.String("A title"),
						TargetBranch:       github.String("TargetBranch"),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "one repository status and optnamespace",
			args: args{
				repoName:         "test-run",
				currentNamespace: "namespace",
				opts: &cli.PacCliOpts{
					Namespace: "optnamespace",
				},
				statuses: []v1alpha1.RepositoryRunStatus{
					{
						Status: v1beta1.Status{
							Conditions: []knativeapis.Condition{
								{
									Reason: "Success",
								},
							},
						},
						CollectedTaskInfos: &map[string]v1alpha1.TaskInfos{},
						PipelineRunName:    "pipelinerun1",
						LogURL:             github.String("https://everywhere.anwywhere"),
						StartTime:          &metav1.Time{Time: cw.Now().Add(-16 * time.Minute)},
						CompletionTime:     &metav1.Time{Time: cw.Now().Add(-15 * time.Minute)},
						SHA:                github.String("SHA"),
						SHAURL:             github.String("https://anurl.com/commit/SHA"),
						Title:              github.String("A title"),
						TargetBranch:       github.String("TargetBranch"),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "multiple repo status",
			args: args{
				opts:             &cli.PacCliOpts{},
				repoName:         "test-run",
				currentNamespace: "namespace",
				statuses: []v1alpha1.RepositoryRunStatus{
					{
						CollectedTaskInfos: &map[string]v1alpha1.TaskInfos{},
						Status: v1beta1.Status{
							Conditions: []knativeapis.Condition{
								{
									Reason: "Success",
								},
							},
						},
						PipelineRunName: "pipelinerun1",
						LogURL:          github.String("https://everywhere.anwywhere"),
						StartTime:       &metav1.Time{Time: cw.Now().Add(-16 * time.Minute)},
						CompletionTime:  &metav1.Time{Time: cw.Now().Add(-15 * time.Minute)},
						SHA:             github.String("SHA"),
						SHAURL:          github.String("https://anurl.com/commit/SHA"),
						Title:           github.String("A title"),
						TargetBranch:    github.String("TargetBranch"),
						EventType:       github.String("pull_request"),
					},
					{
						Status: v1beta1.Status{
							Conditions: []knativeapis.Condition{
								{
									Reason: "Success",
								},
							},
						},
						PipelineRunName: "pipelinerun2",
						LogURL:          github.String("https://everywhere.anwywhere"),
						StartTime:       &metav1.Time{Time: cw.Now().Add(-18 * time.Minute)},
						CompletionTime:  &metav1.Time{Time: cw.Now().Add(-17 * time.Minute)},
						SHA:             github.String("SHA2"),
						SHAURL:          github.String("https://anurl.com/commit/SHA2"),
						Title:           github.String("Another Update"),
						TargetBranch:    github.String("TargetBranch"),
						EventType:       github.String("pull_request"),
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns := tt.args.currentNamespace
			if tt.args.opts.Namespace != "" {
				ns = tt.args.opts.Namespace
			}
			repositories := []*v1alpha1.Repository{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      tt.args.repoName,
						Namespace: ns,
					},
					Spec: v1alpha1.RepositorySpec{
						URL: "https://anurl.com",
					},
					Status: tt.args.statuses,
				},
			}

			tdata := testclient.Data{
				Namespaces: []*corev1.Namespace{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: tt.args.currentNamespace,
						},
					},
				},
				PipelineRuns: tt.args.pruns,
				Repositories: repositories,
			}
			ctx, _ := rtesting.SetupFakeContext(t)
			stdata, _ := testclient.SeedTestData(t, ctx, tdata)
			cs := &params.Run{
				Clients: clients.Clients{
					PipelineAsCode: stdata.PipelineAsCode,
					Tekton:         stdata.Pipeline,
					ConsoleUI:      consoleui.FallBackConsole{},
				},
				Info: info.Info{Kube: info.KubeOpts{Namespace: tt.args.currentNamespace}},
			}

			io, out := tcli.NewIOStream()
			if err := describe(
				ctx, cs, cw, tt.args.opts, io,
				tt.args.repoName); (err != nil) != tt.wantErr {
				t.Errorf("describe() error = %v, wantErr %v", err, tt.wantErr)
			} else {
				golden.Assert(t, out.String(), strings.ReplaceAll(fmt.Sprintf("%s.golden", t.Name()), "/", "-"))
			}
		})
	}
}
