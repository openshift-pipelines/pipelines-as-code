package status

import (
	"context"
	"regexp"
	"strings"

	pacv1alpha1 "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/sort"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

var reasonMessageReplacementRegexp = regexp.MustCompile(`\(image: .*`)

// CollectFailedTasksLogSnippet collects all tasks information we are interested in.
// should really be in a tektoninteractions package but i lack imagination at the moment
func CollectFailedTasksLogSnippet(ctx context.Context, cs *params.Run, kinteract kubeinteraction.Interface, pr *tektonv1beta1.PipelineRun, numLines int64) map[string]pacv1alpha1.TaskInfos {
	failureReasons := map[string]pacv1alpha1.TaskInfos{}
	if pr == nil {
		return failureReasons
	}

	trStatus := sort.GetStatusFromTaskStatusOrFromAsking(ctx, pr, cs)
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
					// see if a pattern match from errRe
					ti.LogSnippet = strings.TrimSpace(log)
				}
			}
		}
		failureReasons[task.PipelineTaskName] = ti
	}
	return failureReasons
}
