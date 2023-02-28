package consoleui

import (
	"context"
	"fmt"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/templates"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"k8s.io/client-go/dynamic"
)

type CustomConsole struct {
	info *info.Info
}

func (o *CustomConsole) GetName() string {
	if o.info.Pac.CustomConsoleName == "" {
		return "Not configured"
	}
	return o.info.Pac.CustomConsoleName
}

func (o *CustomConsole) URL() string {
	if o.info.Pac.CustomConsoleURL == "" {
		return fmt.Sprintf("https://setting.%s.is.not.configured", settings.CustomConsoleURLKey)
	}
	return o.info.Pac.CustomConsoleURL
}

func (o *CustomConsole) DetailURL(pr *tektonv1.PipelineRun) string {
	if o.info.Pac.CustomConsolePRdetail == "" {
		return fmt.Sprintf("https://setting.%s.is.not.configured", settings.CustomConsolePRDetailKey)
	}
	return templates.ReplacePlaceHoldersVariables(o.info.Pac.CustomConsolePRdetail, map[string]string{
		"namespace": pr.GetNamespace(),
		"pr":        pr.GetName(),
	})
}

func (o *CustomConsole) TaskLogURL(pr *tektonv1.PipelineRun, taskRunStatus *tektonv1.PipelineRunTaskRunStatus) string {
	if o.info.Pac.CustomConsolePRTaskLog == "" {
		return fmt.Sprintf("https://setting.%s.is.not.configured", settings.CustomConsolePRTaskLogKey)
	}
	firstFailedStep := ""
	// search for the first failed steps in taskrunstatus
	for _, step := range taskRunStatus.Status.Steps {
		if step.Terminated != nil && step.Terminated.ExitCode != 0 {
			firstFailedStep = step.Name
			break
		}
	}
	return templates.ReplacePlaceHoldersVariables(o.info.Pac.CustomConsolePRTaskLog, map[string]string{
		"namespace":       pr.GetNamespace(),
		"pr":              pr.GetName(),
		"task":            taskRunStatus.PipelineTaskName,
		"pod":             taskRunStatus.Status.PodName,
		"firstFailedStep": firstFailedStep,
	})
}

func (o *CustomConsole) UI(_ context.Context, _ dynamic.Interface) error {
	return nil
}
