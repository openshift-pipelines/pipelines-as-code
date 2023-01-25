package formatting

import (
	"testing"

	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	knativeduckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestPipelineRunStatus(t *testing.T) {
	tests := []struct {
		name string
		pr   *tektonv1beta1.PipelineRun
	}{
		{
			name: "success",
			pr: &tektonv1beta1.PipelineRun{
				Status: tektonv1beta1.PipelineRunStatus{
					Status: knativeduckv1.Status{
						Conditions: knativeduckv1.Conditions{
							{
								Status:  corev1.ConditionTrue,
								Reason:  tektonv1beta1.PipelineRunReasonSuccessful.String(),
								Message: "Completed",
							},
						},
					},
				},
			},
		},
		{
			name: "failure",
			pr: &tektonv1beta1.PipelineRun{
				Status: tektonv1beta1.PipelineRunStatus{
					Status: knativeduckv1.Status{
						Conditions: knativeduckv1.Conditions{
							{
								Status:  corev1.ConditionFalse,
								Reason:  tektonv1beta1.PipelineRunReasonSuccessful.String(),
								Message: "Completed",
							},
						},
					},
				},
			},
		},
		{
			name: "neutral",
			pr: &tektonv1beta1.PipelineRun{
				Status: tektonv1beta1.PipelineRunStatus{
					Status: knativeduckv1.Status{
						Conditions: knativeduckv1.Conditions{},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := PipelineRunStatus(tt.pr)
			assert.Equal(t, output, tt.name, "PipelineRunStatus() = %v, want %v", output, tt.name)
		})
	}
}
