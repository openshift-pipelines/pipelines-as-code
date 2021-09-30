package kubeinteraction

import (
	"fmt"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativeapi "knative.dev/pkg/apis"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestInteraction_CleanupPipelines(t *testing.T) {
	ns := "namespace"
	clock := clockwork.NewFakeClock()

	type args struct {
		namespace      string
		repositoryName string
		maxKeep        int
		pruns          []*v1beta1.PipelineRun
		kept           int
	}

	const cleanupRepoName = "clean-me-up-before-you-go-go"
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "cleanup",
			args: args{
				namespace:      ns,
				repositoryName: cleanupRepoName,
				maxKeep:        1,
				kept:           1,
				pruns: []*v1beta1.PipelineRun{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "pipeline-newest",
							Namespace: ns,
							Labels: map[string]string{
								"pipelinesascode.tekton.dev/repository": cleanupRepoName,
							},
						},
						Status: v1beta1.PipelineRunStatus{
							PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
								StartTime:      &metav1.Time{Time: clock.Now().Add(-5 * time.Minute)},
								CompletionTime: &metav1.Time{Time: clock.Now().Add(10 * time.Minute)},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "pipeline-middest",
							Namespace: ns,
							Labels: map[string]string{
								"pipelinesascode.tekton.dev/repository": cleanupRepoName,
							},
						},
						Status: v1beta1.PipelineRunStatus{
							PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
								StartTime:      &metav1.Time{Time: clock.Now().Add(-15 * time.Minute)},
								CompletionTime: &metav1.Time{Time: clock.Now().Add(20 * time.Minute)},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "pipeline-oldest",
							Namespace: ns,
							Labels: map[string]string{
								"pipelinesascode.tekton.dev/repository": cleanupRepoName,
							},
						},
						Status: v1beta1.PipelineRunStatus{
							PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
								StartTime:      &metav1.Time{Time: clock.Now().Add(-25 * time.Minute)},
								CompletionTime: &metav1.Time{Time: clock.Now().Add(30 * time.Minute)},
							},
						},
					},
				},
			},
		},
		{
			name: "cleanup-skip-running",
			args: args{
				namespace:      ns,
				repositoryName: cleanupRepoName,
				maxKeep:        1,
				kept:           2,
				pruns: []*v1beta1.PipelineRun{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "pipeline-newest",
							Namespace: ns,
							Labels: map[string]string{
								"pipelinesascode.tekton.dev/repository": cleanupRepoName,
							},
						},
						Status: v1beta1.PipelineRunStatus{

							PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
								// pipeline run started 5 minutes ago
								StartTime: &metav1.Time{Time: clock.Now().Add(-5 * time.Minute)},
								// takes 10 minutes to complete
								CompletionTime: &metav1.Time{Time: clock.Now().Add(10 * time.Minute)},
							},
						},
					},

					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "pipeline-oldest",
							Namespace: ns,
							Labels: map[string]string{
								"pipelinesascode.tekton.dev/repository": cleanupRepoName,
							},
						},
						Status: v1beta1.PipelineRunStatus{
							PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
								// pipeline run started 5 minutes ago
								StartTime: &metav1.Time{Time: clock.Now().Add(-25 * time.Minute)},
								// takes 10 minutes to complete
								CompletionTime: &metav1.Time{Time: clock.Now().Add(30 * time.Minute)},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "pipeline-midest",
							Namespace: ns,
							Labels: map[string]string{
								"pipelinesascode.tekton.dev/repository": cleanupRepoName,
							},
						},
						Status: v1beta1.PipelineRunStatus{
							Status: duckv1beta1.Status{
								Conditions: duckv1beta1.Conditions{
									{
										Type:   knativeapi.ConditionSucceeded,
										Status: corev1.ConditionTrue,
										Reason: v1beta1.PipelineRunReasonRunning.String(),
									},
								},
							},
							PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
								// pipeline run started 5 minutes ago
								StartTime: &metav1.Time{Time: clock.Now().Add(-15 * time.Minute)},
								// takes 10 minutes to complete
								CompletionTime: &metav1.Time{Time: clock.Now().Add(20 * time.Minute)},
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			tdata := testclient.Data{
				PipelineRuns: tt.args.pruns,
				Namespaces: []*corev1.Namespace{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: ns,
						},
					},
				},
			}
			stdata, _ := testclient.SeedTestData(t, ctx, tdata)
			observer, _ := zapobserver.New(zap.InfoLevel)
			fakelogger := zap.New(observer).Sugar()
			kint := Interaction{
				Run: &params.Run{
					Clients: clients.Clients{
						Kube:   stdata.Kube,
						Log:    fakelogger,
						Tekton: stdata.Pipeline,
					},
				},
			}

			if err := kint.CleanupPipelines(ctx, tt.args.namespace, tt.args.repositoryName,
				tt.args.maxKeep); (err != nil) != tt.wantErr {
				t.Errorf("CleanupPipelines() error = %v, wantErr %v", err, tt.wantErr)
			}

			plist, err := kint.Run.Clients.Tekton.TektonV1beta1().PipelineRuns(tt.args.namespace).List(ctx,
				metav1.ListOptions{})
			assert.NilError(t, err)
			assert.Equal(t, len(plist.Items), tt.args.kept)
			assert.Equal(t, plist.Items[len(plist.Items)-1].Name, tt.args.pruns[0].Name,
				fmt.Sprintf("%s != %s", plist.Items[0].Name, tt.args.pruns[0].Name))
		})
	}
}
