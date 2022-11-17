package status

import (
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/test/kubernetestint"
	tektontest "github.com/openshift-pipelines/pipelines-as-code/pkg/test/tekton"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativeapi "knative.dev/pkg/apis"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestCollectFailedTasksLogSnippet(t *testing.T) {
	clock := clockwork.NewFakeClock()

	tests := []struct {
		name            string
		message, status string
		wantFailure     int
		podOutput       string
	}{
		{
			name:        "no failures",
			status:      "Success",
			message:     "never gonna make you fail",
			wantFailure: 0,
		},
		{
			name:        "failure pod output",
			status:      "Failed",
			message:     "i am gonna to make you fail",
			podOutput:   "hahah i am the devil of the pod",
			wantFailure: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exitcode := int32(0)
			if tt.wantFailure > 0 {
				exitcode = 1
			}

			pr := tektontest.MakePRCompletion(clock, "pipeline-newest", "ns",
				tektonv1beta1.PipelineRunReasonSuccessful.String(), make(map[string]string), 10)
			pr.Status.TaskRuns = map[string]*tektonv1beta1.PipelineRunTaskRunStatus{
				"task1": {
					PipelineTaskName: "task1",
					Status: &tektonv1beta1.TaskRunStatus{
						TaskRunStatusFields: tektonv1beta1.TaskRunStatusFields{
							PodName:        "pod1",
							StartTime:      &metav1.Time{Time: clock.Now().Add(1 * time.Minute)},
							CompletionTime: &metav1.Time{Time: clock.Now().Add(2 * time.Minute)},
							Steps: []tektonv1beta1.StepState{
								{
									Name: "step1",
									ContainerState: corev1.ContainerState{
										Terminated: &corev1.ContainerStateTerminated{
											ExitCode: exitcode,
										},
									},
								},
							},
						},
						Status: duckv1beta1.Status{
							Conditions: duckv1beta1.Conditions{
								{
									Type:    knativeapi.ConditionSucceeded,
									Status:  corev1.ConditionTrue,
									Reason:  tt.status,
									Message: tt.message,
								},
							},
						},
					},
				},
			}

			ctx, _ := rtesting.SetupFakeContext(t)
			cs := &params.Run{}
			intf := &kubernetestint.KinterfaceTest{}
			if tt.podOutput != "" {
				intf.GetPodLogsOutput = map[string]string{
					"pod1": tt.podOutput,
				}
			}
			got := CollectFailedTasksLogSnippet(ctx, cs, intf, pr, 1)
			assert.Equal(t, tt.wantFailure, len(got))
			if tt.podOutput != "" {
				assert.Equal(t, tt.podOutput, got["task1"].LogSnippet)
			}
		})
	}
}
