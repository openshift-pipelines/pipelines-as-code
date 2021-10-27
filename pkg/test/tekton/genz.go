package tekton

import (
	"time"

	"github.com/jonboulle/clockwork"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"

	// "gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

func MakePrTrStatus(ptaskname string, completionmn int) *tektonv1beta1.PipelineRunTaskRunStatus {
	clock := clockwork.NewFakeClock()

	completionTime := &metav1.Time{}
	if completionmn > 0 {
		completionTime = &metav1.Time{Time: clock.Now().Add(time.Duration(completionmn+20) * time.Minute)}
	}

	conditionTrue := duckv1beta1.Status{
		Conditions: duckv1beta1.Conditions{
			{
				Status: corev1.ConditionTrue,
			},
		},
	}
	return &tektonv1beta1.PipelineRunTaskRunStatus{
		PipelineTaskName: ptaskname,
		Status: &tektonv1beta1.TaskRunStatus{
			TaskRunStatusFields: tektonv1beta1.TaskRunStatusFields{
				StartTime:      &metav1.Time{Time: clock.Now().Add(time.Duration(completionmn+10) * time.Minute)},
				CompletionTime: completionTime,
			},
			Status: conditionTrue,
		},
		ConditionChecks: map[string]*tektonv1beta1.PipelineRunConditionCheckStatus{},
		WhenExpressions: []tektonv1beta1.WhenExpression{},
	}
}

func MakePR(namespace, name string, trstatus map[string]*tektonv1beta1.PipelineRunTaskRunStatus) *tektonv1beta1.PipelineRun {
	if trstatus == nil {
		trstatus = map[string]*tektonv1beta1.PipelineRunTaskRunStatus{}
	}
	return &tektonv1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Status: tektonv1beta1.PipelineRunStatus{
			PipelineRunStatusFields: tektonv1beta1.PipelineRunStatusFields{
				TaskRuns: trstatus,
			},
		},
	}
}
