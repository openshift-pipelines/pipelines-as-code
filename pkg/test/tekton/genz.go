package tekton

import (
	"time"

	"github.com/jonboulle/clockwork"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativeapi "knative.dev/pkg/apis"
	knativeduckv1 "knative.dev/pkg/apis/duck/v1"
)

func MakePrTrStatus(ptaskname, displayName string, completionmn int) *tektonv1.PipelineRunTaskRunStatus {
	clock := clockwork.NewFakeClock()

	completionTime := &metav1.Time{}
	if completionmn > 0 {
		completionTime = &metav1.Time{Time: clock.Now().Add(time.Duration(completionmn+20) * time.Minute)}
	}

	conditionTrue := knativeduckv1.Status{
		Conditions: knativeduckv1.Conditions{
			{
				Status: corev1.ConditionTrue,
			},
		},
	}
	return &tektonv1.PipelineRunTaskRunStatus{
		PipelineTaskName: ptaskname,
		Status: &tektonv1.TaskRunStatus{
			TaskRunStatusFields: tektonv1.TaskRunStatusFields{
				StartTime:      &metav1.Time{Time: clock.Now().Add(time.Duration(completionmn+10) * time.Minute)},
				CompletionTime: completionTime,
				TaskSpec: &tektonv1.TaskSpec{
					DisplayName: displayName,
				},
			},
			Status: conditionTrue,
		},
		WhenExpressions: []tektonv1.WhenExpression{},
	}
}

func MakeChildStatusReference(name string) tektonv1.ChildStatusReference {
	return tektonv1.ChildStatusReference{
		Name: name,
	}
}

func MakePRStatus(namespace, name string, childStatus []tektonv1.ChildStatusReference, status *knativeduckv1.Status) *tektonv1.PipelineRun {
	if status == nil {
		status = &knativeduckv1.Status{}
	}
	return &tektonv1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Status: tektonv1.PipelineRunStatus{
			Status: *status,
			PipelineRunStatusFields: tektonv1.PipelineRunStatusFields{
				ChildReferences: childStatus,
			},
		},
	}
}

func MakePR(name string, annotations map[string]string, spec tektonv1.PipelineRunSpec) *tektonv1.PipelineRun {
	return &tektonv1.PipelineRun{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PipelineRun",
			APIVersion: "tekton.dev/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Annotations: annotations,
		},
		Spec: spec,
	}
}

func MakePRCompletion(clock clockwork.FakeClock, name, namespace, runstatus string, annotations, labels map[string]string, timeshift int) *tektonv1.PipelineRun {
	// fakeing time logic give me headache
	// this will make the pr finish 5mn ago, starting 5-5mn ago
	starttime := time.Duration((timeshift - 5*-1) * int(time.Minute))
	endtime := time.Duration((timeshift * -1) * int(time.Minute))

	statuscondition := corev1.ConditionTrue
	if runstatus == "" {
		runstatus = tektonv1.PipelineRunReasonSuccessful.String()
	} else if runstatus == string(tektonv1.PipelineRunReasonFailed) {
		runstatus = "Failed"
		statuscondition = corev1.ConditionFalse
	}
	if len(annotations) == 0 {
		annotations = labels
	}

	return &tektonv1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Status: tektonv1.PipelineRunStatus{
			Status: knativeduckv1.Status{
				Conditions: knativeduckv1.Conditions{
					{
						Type:   knativeapi.ConditionSucceeded,
						Status: statuscondition,
						Reason: runstatus,
					},
				},
			},
			PipelineRunStatusFields: tektonv1.PipelineRunStatusFields{
				StartTime:      &metav1.Time{Time: clock.Now().Add(starttime)},
				CompletionTime: &metav1.Time{Time: clock.Now().Add(endtime)},
			},
		},
	}
}

func MakePipeline(name string, tasks []tektonv1.PipelineTask, annotations map[string]string) *tektonv1.Pipeline {
	return &tektonv1.Pipeline{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pipeline",
			APIVersion: "tekton.dev/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Annotations: annotations,
		},
		Spec: tektonv1.PipelineSpec{
			Tasks: tasks,
		},
	}
}

func MakeTask(name string, taskSpec tektonv1.TaskSpec) *tektonv1.Task {
	return &tektonv1.Task{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Task",
			APIVersion: "tekton.dev/v1",
		},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       taskSpec,
	}
}

func MakeTaskRunCompletion(clock clockwork.FakeClock, name, namespace, runstatus string, annotation map[string]string, taskStatus tektonv1.TaskRunStatusFields, conditions knativeduckv1.Conditions, timeshift int) *tektonv1.TaskRun {
	starttime := time.Duration((timeshift - 5*-1) * int(time.Minute))
	endtime := time.Duration((timeshift * -1) * int(time.Minute))

	if len(conditions) == 0 {
		conditions = knativeduckv1.Conditions{
			{
				Type:   knativeapi.ConditionSucceeded,
				Status: corev1.ConditionTrue,
				Reason: runstatus,
			},
		}
	}
	taskStatus.StartTime = &metav1.Time{Time: clock.Now().Add(starttime)}
	taskStatus.CompletionTime = &metav1.Time{Time: clock.Now().Add(endtime)}

	return &tektonv1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: annotation,
		},
		Status: tektonv1.TaskRunStatus{
			Status: knativeduckv1.Status{
				Conditions: conditions,
			},
			TaskRunStatusFields: taskStatus,
		},
	}
}
