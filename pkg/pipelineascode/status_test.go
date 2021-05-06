package pipelineascode

import (
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

func TestPipelineRunStatus(t *testing.T) {
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
			pr: tektonv1beta1.PipelineRun{
				Status: tektonv1beta1.PipelineRunStatus{
					Status: duckv1beta1.Status{
						Conditions: duckv1beta1.Conditions{
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

func TestTaskRunListMapSort(t *testing.T) {
	conditionTrue := duckv1beta1.Status{
		Conditions: duckv1beta1.Conditions{
			{
				Status: corev1.ConditionTrue,
			},
		},
	}

	clock := clockwork.NewFakeClock()
	pstatus := func(completionmn int) *tektonv1beta1.PipelineRunTaskRunStatus {
		return &tektonv1beta1.PipelineRunTaskRunStatus{

			PipelineTaskName: "task",
			Status: &tektonv1beta1.TaskRunStatus{
				TaskRunStatusFields: tektonv1beta1.TaskRunStatusFields{
					StartTime:      &metav1.Time{Time: clock.Now().Add(time.Duration(completionmn+10) * time.Minute)},
					CompletionTime: &metav1.Time{Time: clock.Now().Add(time.Duration(completionmn+20) * time.Minute)},
				},
				Status: conditionTrue,
			},
			ConditionChecks: map[string]*tektonv1beta1.PipelineRunConditionCheckStatus{},
			WhenExpressions: []tektonv1beta1.WhenExpression{},
		}
	}

	status := map[string]*tektonv1beta1.PipelineRunTaskRunStatus{
		"first":  pstatus(5),
		"last":   pstatus(15),
		"middle": pstatus(10),
	}

	trlist := newTaskrunListFromMap(status)
	sort.Sort(trlist)
	assert.Equal(t, trlist[0].TaskrunName, "last")
	assert.Equal(t, trlist[1].TaskrunName, "middle")
	assert.Equal(t, trlist[2].TaskrunName, "first")

	// Not sorting the middle one since no status, comes first then
	status["middle"].Status.TaskRunStatusFields.StartTime = nil
	trlist = newTaskrunListFromMap(status)
	sort.Sort(trlist)
	assert.Equal(t, trlist[0].TaskrunName, "middle")

	// Only last become sorted because middle and first has been removed
	status["first"].Status.TaskRunStatusFields.StartTime = nil
	trlist = newTaskrunListFromMap(status)
	sort.Sort(trlist)
	assert.Equal(t, trlist[len(trlist)-1].TaskrunName, "last")
}

func TestConditionEmoji(t *testing.T) {
	tests := []struct {
		name      string
		condition duckv1beta1.Conditions
		substr    string
	}{
		{
			name: "failed",
			condition: duckv1beta1.Conditions{
				{
					Status: corev1.ConditionFalse,
				},
			},
			substr: "Failed",
		},
		{
			name: "success",
			condition: duckv1beta1.Conditions{
				{
					Status: corev1.ConditionTrue,
				},
			},
			substr: "Succeeded",
		},
		{
			name: "Running",
			condition: duckv1beta1.Conditions{
				{
					Status: corev1.ConditionUnknown,
				},
			},
			substr: "Running",
		},
		{
			name:      "None",
			condition: duckv1beta1.Conditions{},
			substr:    "---",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConditionEmoji(tt.condition)
			assert.Assert(t, strings.Contains(got, tt.substr))
		})
	}
}
