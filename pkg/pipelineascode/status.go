package pipelineascode

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"text/template"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"
	"github.com/tektoncd/cli/pkg/formatted"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knative1 "knative.dev/pkg/apis/duck/v1beta1"
)

const checkStatustmpl = `{{.taskStatus}}

<hr>

🗒️ Full log available here <a href="{{.consoleURL}}">here</a>.
`

const taskStatustmpl = `
<table>
  <tr><th>Status</th><th>Duration</th><th>Name</th></tr>

{{- range $taskrun := .TaskRunList }}
<tr>
<td>{{ formatCondition $taskrun.Status.Conditions }}</td>
<td>{{ formatDuration $taskrun.Status.StartTime $taskrun.Status.CompletionTime }}</td><td>

{{ $taskrun.ConsoleLogURL }}

</td></tr>
{{- end }}
</table>`

type tkr struct {
	TaskrunName string
	LogURL      string
	*tektonv1beta1.PipelineRunTaskRunStatus
}

func (t tkr) ConsoleLogURL() string {
	return fmt.Sprintf("[%s](%s/%s)", t.PipelineTaskName, t.LogURL, t.PipelineTaskName)
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

func newTaskrunListFromMap(statusMap map[string]*tektonv1beta1.PipelineRunTaskRunStatus, consoleURL string) taskrunList {
	trl := taskrunList{}
	for taskrunName, taskrunStatus := range statusMap {
		trl = append(trl, tkr{
			taskrunName,
			consoleURL,
			taskrunStatus,
		})
	}
	return trl
}

// pipelineRunStatus return status of PR  success failed or skipped
func pipelineRunStatus(pr *tektonv1beta1.PipelineRun) string {
	if len(pr.Status.Conditions) == 0 {
		return "neutral"
	}
	if pr.Status.Conditions[0].Status == corev1.ConditionFalse {
		return "failure"
	}
	return "success"
}

func ConditionEmoji(c knative1.Conditions) string {
	var status string
	if len(c) == 0 {
		return "---"
	}

	// TODO: there is other weird errors we need to handle.

	switch c[0].Status {
	case corev1.ConditionFalse:
		return "❌ Failed"
	case corev1.ConditionTrue:
		return "✅ Succeeded"
	case corev1.ConditionUnknown:
		return "🏃 Running"
	}

	return status
}

func statusOfAllTaskListForCheckRun(pr *tektonv1beta1.PipelineRun, consoleURL string) (string, error) {
	var trl taskrunList
	var outputBuffer bytes.Buffer

	if len(pr.Status.TaskRuns) != 0 {
		trl = newTaskrunListFromMap(pr.Status.TaskRuns, consoleURL)
		sort.Sort(sort.Reverse(trl))
	}

	funcMap := template.FuncMap{
		"formatDuration":  formatted.Duration,
		"formatCondition": ConditionEmoji,
	}

	data := struct {
		TaskRunList taskrunList
	}{
		TaskRunList: trl,
	}

	t := template.Must(template.New("Task Status").Funcs(funcMap).Parse(taskStatustmpl))
	if err := t.Execute(&outputBuffer, data); err != nil {
		fmt.Fprintf(&outputBuffer, "failed to execute template: ")
		return "", err
	}

	return outputBuffer.String(), nil
}

func postFinalStatus(ctx context.Context, cs *cli.Clients, k8int cli.KubeInteractionIntf, runinfo *webvcs.RunInfo, prName, namespace string) (*tektonv1beta1.PipelineRun, error) {
	var outputBuffer bytes.Buffer

	pr, err := cs.Tekton.TektonV1beta1().PipelineRuns(namespace).Get(ctx, prName, v1.GetOptions{})
	if err != nil {
		return pr, err
	}

	consoleURL, err := k8int.GetConsoleUI(ctx, namespace, pr.Name)
	if err != nil {
		consoleURL = "https://giphy.com/search/cat-reading"
	}

	taskStatus, err := statusOfAllTaskListForCheckRun(pr, consoleURL)
	if err != nil {
		return pr, err
	}

	data := map[string]string{
		"taskStatus": taskStatus,
		"consoleURL": consoleURL,
	}

	t := template.Must(template.New("Pipeline Status").Parse(checkStatustmpl))
	if err := t.Execute(&outputBuffer, data); err != nil {
		fmt.Fprintf(&outputBuffer, "failed to execute template: ")
		return pr, err
	}

	err = createStatus(ctx, cs, runinfo, "completed", pipelineRunStatus(pr),
		outputBuffer.String(), consoleURL, false)

	return pr, err
}
