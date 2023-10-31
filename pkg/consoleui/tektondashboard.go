package consoleui

import (
	"context"
	"fmt"

	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"k8s.io/client-go/dynamic"
)

type TektonDashboard struct {
	BaseURL string
}

const tektonDashboardName = "Tekton Dashboard"

func (t *TektonDashboard) GetName() string {
	return tektonDashboardName
}

func (t *TektonDashboard) DetailURL(pr *tektonv1.PipelineRun) string {
	return fmt.Sprintf("%s/#/namespaces/%s/pipelineruns/%s", t.BaseURL, pr.GetNamespace(), pr.GetName())
}

func (t *TektonDashboard) NamespaceURL(pr *tektonv1.PipelineRun) string {
	return fmt.Sprintf("%s/#/namespaces/%s/pipelineruns", t.BaseURL, pr.GetNamespace())
}

func (t *TektonDashboard) TaskLogURL(pr *tektonv1.PipelineRun, taskRunStatus *tektonv1.PipelineRunTaskRunStatus) string {
	return fmt.Sprintf("%s?pipelineTask=%s", t.DetailURL(pr), taskRunStatus.PipelineTaskName)
}

func (t *TektonDashboard) URL() string {
	return t.BaseURL
}

func (t *TektonDashboard) UI(_ context.Context, _ dynamic.Interface) error {
	return nil
}

func (t *TektonDashboard) SetParams(_ map[string]string) {
}
