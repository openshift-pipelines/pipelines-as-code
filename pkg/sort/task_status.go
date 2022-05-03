package sort

import (
	"bytes"
	"fmt"
	"sort"
	"text/template"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/consoleui"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/formatting"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

type tkr struct {
	taskLogURL string
	*tektonv1beta1.PipelineRunTaskRunStatus
}

func (t tkr) ConsoleLogURL() string {
	return fmt.Sprintf("[%s](%s)", t.PipelineTaskName, t.taskLogURL)
}

type taskrunList []tkr

func (trs taskrunList) Len() int      { return len(trs) }
func (trs taskrunList) Swap(i, j int) { trs[i], trs[j] = trs[j], trs[i] }
func (trs taskrunList) Less(i, j int) bool {
	if trs[j].Status == nil || trs[j].Status.StartTime == nil {
		return false
	}

	if trs[i].Status == nil || trs[i].Status.StartTime == nil {
		return true
	}

	return trs[j].Status.StartTime.Before(trs[i].Status.StartTime)
}

// TaskStatusTmpl generate a template of all status of a taskruns sorted to a statusTemplate as defined by the git provider
func TaskStatusTmpl(pr *tektonv1beta1.PipelineRun, console consoleui.Interface, statusTemplate string) (string, error) {
	trl := taskrunList{}
	outputBuffer := bytes.Buffer{}

	if len(pr.Status.TaskRuns) == 0 {
		return "PipelineRun has no taskruns", nil
	}

	for _, taskrunStatus := range pr.Status.TaskRuns {
		trl = append(trl, tkr{
			taskLogURL: console.TaskLogURL(
				pr.GetNamespace(),
				pr.GetName(),
				taskrunStatus.PipelineTaskName,
			),
			PipelineRunTaskRunStatus: taskrunStatus,
		})
	}
	sort.Sort(sort.Reverse(trl))

	funcMap := template.FuncMap{
		"formatDuration":  formatting.Duration,
		"formatCondition": formatting.ConditionEmoji,
	}

	data := struct{ TaskRunList taskrunList }{TaskRunList: trl}

	t := template.Must(template.New("Task Status").Funcs(funcMap).Parse(statusTemplate))
	if err := t.Execute(&outputBuffer, data); err != nil {
		fmt.Fprintf(&outputBuffer, "failed to execute template: ")
		return "", err
	}

	return outputBuffer.String(), nil
}
