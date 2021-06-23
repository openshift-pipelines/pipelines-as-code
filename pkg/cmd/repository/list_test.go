package repository

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v35/github"
	"github.com/jonboulle/clockwork"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/flags"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	"gotest.tools/v3/golden"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativeapis "knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck/v1beta1"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestList(t *testing.T) {
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
	repoNamespace1 := &v1alpha1.Repository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "repo1",
			Namespace: namespace1.GetName(),
		},
		Spec: v1alpha1.RepositorySpec{
			Namespace: namespace1.GetName(),
			URL:       "https://anurl.com/owner/repo",
			Branch:    "branch",
			EventType: "pull_request",
		},
		Status: []v1alpha1.RepositoryRunStatus{
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
				Title:           github.String("A title"),
			},
		},
	}
	repoNamespace2 := &v1alpha1.Repository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "repo2",
			Namespace: namespace2.GetName(),
		},
		Spec: v1alpha1.RepositorySpec{
			Namespace: namespace2.GetName(),
			URL:       "https://anurl.com/owner/repo",
			Branch:    "branch",
			EventType: "pull_request",
		},
		Status: []v1alpha1.RepositoryRunStatus{
			{
				Status: v1beta1.Status{
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
				Title:           github.String("A title"),
			},
		},
	}

	type args struct {
		namespaces       []*corev1.Namespace
		repositories     []*v1alpha1.Repository
		currentNamespace string
		opts             *flags.CliOpts
		selectors        string
		noheaders        bool
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Test list repositories",
			args: args{
				opts:             &flags.CliOpts{},
				currentNamespace: namespace1.GetName(),
				namespaces: []*corev1.Namespace{
					namespace1,
				},
				repositories: []*v1alpha1.Repository{repoNamespace1},
			},
		},
		{
			name: "Test list repositories all namespaces",
			args: args{
				opts:             &flags.CliOpts{AllNameSpaces: true},
				currentNamespace: "namespace",
				namespaces:       []*corev1.Namespace{namespace1, namespace2},
				repositories:     []*v1alpha1.Repository{repoNamespace1, repoNamespace2},
			},
		},
		{
			name: "Test list repositories specific namespaces",
			args: args{
				opts:             &flags.CliOpts{},
				currentNamespace: namespace2.GetName(),
				namespaces:       []*corev1.Namespace{namespace1, namespace2},
				repositories:     []*v1alpha1.Repository{repoNamespace1, repoNamespace2},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tdata := testclient.Data{
				Namespaces:   tt.args.namespaces,
				Repositories: tt.args.repositories,
			}
			ctx, _ := rtesting.SetupFakeContext(t)
			stdata, _ := testclient.SeedTestData(t, ctx, tdata)
			cs := &cli.Clients{
				PipelineAsCode: stdata.PipelineAsCode,
			}
			io, out := newIOStream()
			if err := list(ctx, cs, tt.args.opts, io, tt.args.currentNamespace,
				tt.args.selectors, tt.args.noheaders); (err != nil) != tt.wantErr {
				t.Errorf("describe() error = %v, wantErr %v", err, tt.wantErr)
			} else {
				golden.Assert(t, out.String(), strings.ReplaceAll(fmt.Sprintf("%s.golden", t.Name()), "/", "-"))
			}
		})
	}
}
