package list

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v47/github"
	"github.com/jonboulle/clockwork"
	pacv1alpha1 "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/consoleui"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	tektontest "github.com/openshift-pipelines/pipelines-as-code/pkg/test/tekton"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"gotest.tools/v3/golden"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativeapis "knative.dev/pkg/apis"
	kv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func newIOStream() (*cli.IOStreams, *bytes.Buffer) {
	in := &bytes.Buffer{}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	return &cli.IOStreams{
		In:     io.NopCloser(in),
		Out:    out,
		ErrOut: errOut,
	}, out
}

func TestList(t *testing.T) {
	running := tektonv1beta1.PipelineRunReasonRunning.String()
	cw := clockwork.NewFakeClock()
	namespace1 := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "namespace1",
		},
	}
	namespace2 := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "namespace2",
		},
	}
	repoNamespace1SHA := "abcd2"
	repoNamespace1 := &pacv1alpha1.Repository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "repo1",
			Namespace: namespace1.GetName(),
		},
		Spec: pacv1alpha1.RepositorySpec{
			URL: "https://anurl.com/owner/repo",
		},
		Status: []pacv1alpha1.RepositoryRunStatus{
			{
				Status: kv1beta1.Status{
					Conditions: []knativeapis.Condition{
						{
							Reason: "Success",
						},
					},
				},
				PipelineRunName: "pipelinerun1",
				StartTime:       &metav1.Time{Time: cw.Now().Add(-16 * time.Minute)},
				CompletionTime:  &metav1.Time{Time: cw.Now().Add(-15 * time.Minute)},
				SHA:             github.String(repoNamespace1SHA),
				SHAURL:          github.String("https://somewhereandnowhere/1"),
				Title:           github.String("A title"),
				LogURL:          github.String("https://help.me.obiwan.kenobi/1"),
			},
		},
	}
	repoNamespace2 := &pacv1alpha1.Repository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "repo2",
			Namespace: namespace2.GetName(),
		},
		Spec: pacv1alpha1.RepositorySpec{
			URL: "https://anurl.com/owner/repo",
		},
		Status: []pacv1alpha1.RepositoryRunStatus{
			{
				Status: kv1beta1.Status{
					Conditions: []knativeapis.Condition{
						{
							Reason: "Success",
						},
					},
				},
				PipelineRunName: "pipelinerun2",
				StartTime:       &metav1.Time{Time: cw.Now().Add(-16 * time.Minute)},
				CompletionTime:  &metav1.Time{Time: cw.Now().Add(-15 * time.Minute)},
				SHA:             github.String("SHA"),
				SHAURL:          github.String("https://somewhereandnowhere/2"),
				Title:           github.String("A title"),
				LogURL:          github.String("https://help.me.obiwan.kenobi"),
			},
		},
	}

	type args struct {
		namespaces       []*corev1.Namespace
		repositories     []*pacv1alpha1.Repository
		pipelineruns     []*tektonv1beta1.PipelineRun
		currentNamespace string
		opts             *cli.PacCliOpts
		selectors        string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Test list repositories only",
			args: args{
				opts:             &cli.PacCliOpts{},
				currentNamespace: namespace1.GetName(),
				namespaces: []*corev1.Namespace{
					namespace1,
				},
				repositories: []*pacv1alpha1.Repository{repoNamespace1},
			},
		},
		{
			name: "Test list repositories only all namespaces",
			args: args{
				opts:             &cli.PacCliOpts{AllNameSpaces: true},
				currentNamespace: "namespace",
				namespaces:       []*corev1.Namespace{namespace1, namespace2},
				repositories:     []*pacv1alpha1.Repository{repoNamespace1, repoNamespace2},
			},
		},
		{
			name: "Test list repositories only specific namespaces",
			args: args{
				opts:             &cli.PacCliOpts{},
				currentNamespace: namespace2.GetName(),
				namespaces:       []*corev1.Namespace{namespace1, namespace2},
				repositories:     []*pacv1alpha1.Repository{repoNamespace1, repoNamespace2},
			},
		},
		{
			name: "Test with real time",
			args: args{
				opts:             &cli.PacCliOpts{UseRealTime: true},
				currentNamespace: namespace2.GetName(),
				namespaces:       []*corev1.Namespace{namespace1, namespace2},
				repositories:     []*pacv1alpha1.Repository{repoNamespace1, repoNamespace2},
			},
		},
		{
			name: "Test list repositories only live PR",
			args: args{
				opts:             &cli.PacCliOpts{},
				currentNamespace: namespace1.GetName(),
				namespaces: []*corev1.Namespace{
					namespace1,
				},
				repositories: []*pacv1alpha1.Repository{repoNamespace1},
				pipelineruns: []*tektonv1beta1.PipelineRun{
					tektontest.MakePRCompletion(cw, "running", namespace1.GetName(), running,
						map[string]string{
							"pipelinesascode.tekton.dev/repository": repoNamespace1.GetName(),
							"pipelinesascode.tekton.dev/sha":        repoNamespace1SHA,
						}, 30),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tdata := testclient.Data{
				Namespaces:   tt.args.namespaces,
				Repositories: tt.args.repositories,
				PipelineRuns: tt.args.pipelineruns,
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
			io, out := newIOStream()
			if err := list(ctx, cs, tt.args.opts, io,
				cw, tt.args.selectors); (err != nil) != tt.wantErr {
				t.Errorf("describe() error = %v, wantErr %v", err, tt.wantErr)
			} else {
				golden.Assert(t, out.String(), strings.ReplaceAll(fmt.Sprintf("%s.golden", t.Name()), "/", "-"))
			}
		})
	}
}
