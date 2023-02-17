package status

import (
	"testing"

	"github.com/jonboulle/clockwork"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	paramclients "github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/test/kubernetestint"
	tektontest "github.com/openshift-pipelines/pipelines-as-code/pkg/test/tekton"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	knativeapi "knative.dev/pkg/apis"
	knativeduckv1 "knative.dev/pkg/apis/duck/v1"
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

			pr := tektontest.MakePRCompletion(clock, "pipeline-newest", "ns", tektonv1.PipelineRunReasonSuccessful.String(), make(map[string]string), 10)
			pr.Status.ChildReferences = []tektonv1.ChildStatusReference{
				{
					TypeMeta: runtime.TypeMeta{
						Kind: "TaskRun",
					},
					Name:             "task1",
					PipelineTaskName: "task1",
				},
			}

			taskStatus := tektonv1.TaskRunStatusFields{
				PodName: "task1",
				Steps: []tektonv1.StepState{
					{
						Name: "step1",
						ContainerState: corev1.ContainerState{
							Terminated: &corev1.ContainerStateTerminated{
								ExitCode: exitcode,
							},
						},
					},
				},
			}

			tdata := testclient.Data{
				TaskRuns: []*tektonv1.TaskRun{
					tektontest.MakeTaskRunCompletion(clock, "task1", "ns", "pipeline-newest",
						map[string]string{}, taskStatus, knativeduckv1.Conditions{
							{
								Type:    knativeapi.ConditionSucceeded,
								Status:  corev1.ConditionTrue,
								Reason:  tt.status,
								Message: tt.message,
							},
						},
						10),
				},
			}
			ctx, _ := rtesting.SetupFakeContext(t)
			stdata, _ := testclient.SeedTestData(t, ctx, tdata)
			cs := &params.Run{Clients: paramclients.Clients{
				Tekton: stdata.Pipeline,
			}}
			intf := &kubernetestint.KinterfaceTest{}
			if tt.podOutput != "" {
				intf.GetPodLogsOutput = map[string]string{
					"task1": tt.podOutput,
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

func TestGetStatusFromTaskStatusOrFromAsking(t *testing.T) {
	testNS := "test"
	tests := []struct {
		name               string
		pr                 *tektonv1.PipelineRun
		numStatus          int
		expectedLogSnippet string
		taskRuns           []*tektonv1.TaskRun
	}{
		{
			name:      "get status from child references post tektoncd/pipelines 0.44",
			numStatus: 2,
			pr: &tektonv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testNS,
				},
				Status: tektonv1.PipelineRunStatus{
					PipelineRunStatusFields: tektonv1.PipelineRunStatusFields{
						ChildReferences: []tektonv1.ChildStatusReference{
							{
								TypeMeta: runtime.TypeMeta{
									Kind: "TaskRun",
								},
								Name: "hello",
							},
							{
								TypeMeta: runtime.TypeMeta{
									Kind: "TaskRun",
								},
								Name: "yolo",
							},
						},
					},
				},
			},
			taskRuns: []*tektonv1.TaskRun{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "hello",
						Namespace: testNS,
					},
					Status: tektonv1.TaskRunStatus{},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "yolo",
						Namespace: testNS,
					},
					Status: tektonv1.TaskRunStatus{},
				},
			},
		},
		{
			name: "error get status from child references post tektoncd/pipelines 0.44",
			pr: &tektonv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testNS,
				},
				Status: tektonv1.PipelineRunStatus{
					PipelineRunStatusFields: tektonv1.PipelineRunStatusFields{
						ChildReferences: []tektonv1.ChildStatusReference{
							{
								Name: "hello",
							},
							{
								Name: "yolo",
							},
						},
					},
				},
			},
			expectedLogSnippet: "cannot get taskrun status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			observer, obslog := zapobserver.New(zap.InfoLevel)
			logger := zap.New(observer).Sugar()
			ctx, _ := rtesting.SetupFakeContext(t)
			run := params.New()

			tdata := testclient.Data{
				Namespaces: []*corev1.Namespace{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: testNS,
						},
					},
				},
				TaskRuns: tt.taskRuns,
			}
			stdata, _ := testclient.SeedTestData(t, ctx, tdata)
			run.Clients = paramclients.Clients{
				Kube:   stdata.Kube,
				Tekton: stdata.Pipeline,
				Log:    logger,
			}
			statuses := GetStatusFromTaskStatusOrFromAsking(ctx, tt.pr, run)
			assert.Equal(t, len(statuses), tt.numStatus)

			if tt.expectedLogSnippet != "" {
				logmsg := obslog.FilterMessageSnippet(tt.expectedLogSnippet).TakeAll()
				assert.Assert(t, len(logmsg) > 0, "log messages", logmsg, tt.expectedLogSnippet)
			}
		})
	}
}
