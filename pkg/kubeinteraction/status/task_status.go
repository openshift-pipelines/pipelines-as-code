package status

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	pacv1alpha1 "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var reasonMessageReplacementRegexp = regexp.MustCompile(`\(image: .*`)

// GetTaskRunStatusForPipelineTask takes a minimal embedded status child reference and returns the actual TaskRunStatus
// for the PipelineTask. It returns an error if the child reference's kind isn't TaskRun.
func GetTaskRunStatusForPipelineTask(ctx context.Context, client versioned.Interface, ns string, childRef tektonv1.ChildStatusReference) (*tektonv1.TaskRunStatus, error) {
	if childRef.Kind != "TaskRun" {
		return nil, fmt.Errorf("could not fetch status for PipelineTask %s: should have kind TaskRun, but is %s", childRef.PipelineTaskName, childRef.Kind)
	}

	tr, err := client.TektonV1().TaskRuns(ns).Get(ctx, childRef.Name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}
	if tr == nil {
		return nil, nil
	}

	return &tr.Status, nil
}

// GetStatusFromTaskStatusOrFromAsking will return the status of the taskruns,
// it would use the embedded one if it's available (pre tekton 0.44.0) or try
// to get it from the child references
func GetStatusFromTaskStatusOrFromAsking(ctx context.Context, pr *tektonv1.PipelineRun, run *params.Run) map[string]*tektonv1.PipelineRunTaskRunStatus {
	trStatus := map[string]*tektonv1.PipelineRunTaskRunStatus{}
	for _, cr := range pr.Status.ChildReferences {
		ts, err := GetTaskRunStatusForPipelineTask(
			ctx, run.Clients.Tekton, pr.GetNamespace(), cr,
		)
		if err != nil {
			run.Clients.Log.Warnf("cannot get taskrun status pr %s ns: %s err: %w", pr.GetName(), pr.GetNamespace(), err)
			continue
		}
		if ts == nil {
			run.Clients.Log.Warnf("cannot get taskrun status pr %s ns: %s, ts come back nil?", pr.GetName(), pr.GetNamespace(), err)
			continue
		}
		// search in taskSpecs if there is a displayName for that status
		if pr.Spec.PipelineSpec != nil && pr.Spec.PipelineSpec.Tasks != nil {
			for _, taskSpec := range pr.Spec.PipelineSpec.Tasks {
				if ts.TaskSpec != nil && taskSpec.Name == cr.PipelineTaskName {
					ts.TaskSpec.DisplayName = taskSpec.DisplayName
				}
			}
		}
		trStatus[cr.Name] = &tektonv1.PipelineRunTaskRunStatus{
			PipelineTaskName: cr.PipelineTaskName,
			Status:           ts,
		}
	}
	return trStatus
}

// CollectFailedTasksLogSnippet collects all tasks information we are interested in.
// should really be in a tektoninteractions package but i lack imagination at the moment
func CollectFailedTasksLogSnippet(ctx context.Context, cs *params.Run, kinteract kubeinteraction.Interface, pr *tektonv1.PipelineRun, numLines int64) map[string]pacv1alpha1.TaskInfos {
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
			DisplayName:    task.Status.TaskSpec.DisplayName,
		}
		if ti.Reason == "TaskRunValidationFailed" || ti.Reason == tektonv1.TaskRunReasonCancelled.String() || ti.Reason == tektonv1.TaskRunReasonTimedOut.String() || ti.Reason == tektonv1.TaskRunReasonImagePullFailed.String() {
			failureReasons[task.PipelineTaskName] = ti
			continue
		} else if ti.Reason != tektonv1.PipelineRunReasonFailed.String() {
			continue
		}

		if kinteract != nil {
			for _, step := range task.Status.Steps {
				if step.Terminated != nil && step.Terminated.ExitCode != 0 {
					log, err := kinteract.GetPodLogs(ctx, pr.GetNamespace(), task.Status.PodName, step.Container, numLines)
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
