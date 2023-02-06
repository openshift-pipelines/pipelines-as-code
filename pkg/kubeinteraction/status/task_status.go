package status

import (
	"context"
	"regexp"
	"strings"

	pacv1alpha1 "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	tektonstatus "github.com/tektoncd/pipeline/pkg/status"
)

var reasonMessageReplacementRegexp = regexp.MustCompile(`\(image: .*`)

// GetStatusFromTaskStatusOrFromAsking will return the status of the taskruns,
// it would use the embedded one if it's available (pre tekton 0.44.0) or try
// to get it from the child references
func GetStatusFromTaskStatusOrFromAsking(ctx context.Context, pr *tektonv1beta1.PipelineRun, run *params.Run) map[string]*tektonv1beta1.PipelineRunTaskRunStatus {
	trStatus := map[string]*tektonv1beta1.PipelineRunTaskRunStatus{}
	if len(pr.Status.TaskRuns) > 0 {
		// Deprecated since pipeline 0.44.0
		return pr.Status.TaskRuns
	}
	for _, cr := range pr.Status.ChildReferences {
		ts, err := tektonstatus.GetTaskRunStatusForPipelineTask(
			ctx, run.Clients.Tekton, pr.GetNamespace(), cr,
		)
		if err != nil {
			run.Clients.Log.Warnf("cannot get taskrun status pr %s ns: %s err: %w", pr.GetName(), pr.GetNamespace(), err)
			continue
		}
		trStatus[cr.Name] = &tektonv1beta1.PipelineRunTaskRunStatus{
			PipelineTaskName: cr.PipelineTaskName,
			Status:           ts,
		}
	}
	return trStatus
}

// CollectFailedTasksLogSnippet collects all tasks information we are interested in.
// should really be in a tektoninteractions package but i lack imagination at the moment
func CollectFailedTasksLogSnippet(ctx context.Context, cs *params.Run, kinteract kubeinteraction.Interface, pr *tektonv1beta1.PipelineRun, numLines int64) map[string]pacv1alpha1.TaskInfos {
	failureReasons := map[string]pacv1alpha1.TaskInfos{}
	if pr == nil {
		return failureReasons
	}

	trStatus := GetStatusFromTaskStatusOrFromAsking(ctx, pr, cs)
	for _, task := range trStatus {
		if task.Status == nil {
			continue
		}
		if len(task.Status.Conditions) == 0 {
			continue
		}
		ti := pacv1alpha1.TaskInfos{
			Name:           task.PipelineTaskName,
			Message:        reasonMessageReplacementRegexp.ReplaceAllString(task.Status.Conditions[0].Message, ""),
			CompletionTime: task.Status.CompletionTime,
			Reason:         task.Status.Conditions[0].Reason,
		}
		if ti.Reason == "TaskRunValidationFailed" || ti.Reason == tektonv1beta1.TaskRunReasonCancelled.String() || ti.Reason == tektonv1beta1.TaskRunReasonTimedOut.String() || ti.Reason == tektonv1beta1.TaskRunReasonImagePullFailed.String() {
			failureReasons[task.PipelineTaskName] = ti
			continue
		} else if ti.Reason != tektonv1beta1.PipelineRunReasonFailed.String() {
			continue
		}

		if kinteract != nil {
			for _, step := range task.Status.Steps {
				if step.Terminated != nil && step.Terminated.ExitCode != 0 {
					log, err := kinteract.GetPodLogs(ctx, pr.GetNamespace(), task.Status.PodName, step.ContainerName, numLines)
					if err != nil {
						cs.Clients.Log.Errorf("cannot get pod logs: %w", err)
						continue
					}
					trimmed := strings.TrimSpace(log)
					if strings.HasSuffix(trimmed, " Skipping step because a previous step failed") {
						continue
					}
					// see if a pattern match from errRe
					ti.LogSnippet = strings.TrimSpace(trimmed)
				}
			}
		}
		failureReasons[task.PipelineTaskName] = ti
	}
	return failureReasons
}
