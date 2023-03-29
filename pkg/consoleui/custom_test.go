package consoleui

import (
	"strings"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCustomGood(t *testing.T) {
	consoleName := "MyCorp Console"
	consoleURL := "https://mycorp.console"
	consolePRdetail := "https://mycorp.console/{{ namespace }}/{{ pr }}"
	consolePRtasklog := "https://mycorp.console/{{ namespace }}/{{ pr }}/{{ task }}/{{ pod }}/{{ firstFailedStep }}"

	c := CustomConsole{
		Info: &info.Info{
			Pac: &info.PacOpts{
				Settings: &settings.Settings{
					CustomConsoleName:      consoleName,
					CustomConsoleURL:       consoleURL,
					CustomConsolePRdetail:  consolePRdetail,
					CustomConsolePRTaskLog: consolePRtasklog,
				},
			},
		},
	}
	pr := &tektonv1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns",
			Name:      "pr",
		},
	}
	trStatus := &tektonv1.PipelineRunTaskRunStatus{
		PipelineTaskName: "task",
		Status: &tektonv1.TaskRunStatus{
			TaskRunStatusFields: tektonv1.TaskRunStatusFields{
				PodName: "pod",
				Steps: []tektonv1.StepState{
					{
						Name: "failure",
						ContainerState: corev1.ContainerState{
							Terminated: &corev1.ContainerStateTerminated{
								ExitCode: 1,
							},
						},
					},
					{
						Name: "nextFailure",
						ContainerState: corev1.ContainerState{
							Terminated: &corev1.ContainerStateTerminated{
								ExitCode: 1,
							},
						},
					},
				},
			},
		},
	}
	assert.Equal(t, c.GetName(), consoleName)
	assert.Equal(t, c.URL(), consoleURL)
	assert.Equal(t, c.DetailURL(pr), "https://mycorp.console/ns/pr")
	assert.Equal(t, c.TaskLogURL(pr, trStatus), "https://mycorp.console/ns/pr/task/pod/failure")
}

func TestCustomBad(t *testing.T) {
	c := CustomConsole{
		Info: &info.Info{
			Pac: &info.PacOpts{
				Settings: &settings.Settings{},
			},
		},
	}
	pr := &tektonv1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns",
			Name:      "pr",
		},
	}
	assert.Assert(t, strings.Contains(c.GetName(), "Not configured"))
	assert.Assert(t, strings.Contains(c.URL(), "is.not.configured"))
	assert.Assert(t, strings.Contains(c.DetailURL(pr), "is.not.configured"))
	assert.Assert(t, strings.Contains(c.TaskLogURL(pr, nil), "is.not.configured"))
}
