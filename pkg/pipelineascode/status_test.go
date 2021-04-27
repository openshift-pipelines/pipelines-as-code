package pipelineascode

import (
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

func Test_pipelineRunStatus(t *testing.T) {
	tests := []struct {
		name string
		pr   tektonv1beta1.PipelineRun
	}{
		{
			name: "success",
			pr: tektonv1beta1.PipelineRun{
				Status: tektonv1beta1.PipelineRunStatus{
					Status: duckv1beta1.Status{
						Conditions: duckv1beta1.Conditions{
							{
								Status:  corev1.ConditionTrue,
								Reason:  v1beta1.PipelineRunReasonSuccessful.String(),
								Message: "Completed",
							},
						},
					},
				},
			},
		},
		{
			name: "failure",
			pr: tektonv1beta1.PipelineRun{
				Status: tektonv1beta1.PipelineRunStatus{
					Status: duckv1beta1.Status{
						Conditions: duckv1beta1.Conditions{
							{
								Status:  corev1.ConditionFalse,
								Reason:  v1beta1.PipelineRunReasonSuccessful.String(),
								Message: "Completed",
							},
						},
					},
				},
			},
		},
		{
			name: "neutral",
			pr: tektonv1beta1.PipelineRun{
				Status: tektonv1beta1.PipelineRunStatus{
					Status: duckv1beta1.Status{
						Conditions: duckv1beta1.Conditions{},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pipelineRunStatus(&tt.pr); got != tt.name {
				t.Errorf("pipelineRunStatus() = %v, want %v", got, tt.name)
			}
		})
	}
}
