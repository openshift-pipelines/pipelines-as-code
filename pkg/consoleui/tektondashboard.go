package consoleui

import (
	"context"
	"fmt"

	"k8s.io/client-go/dynamic"
)

type TektonDashboard struct {
	BaseURL string
}

const tektonDashboardName = "Tekton Dashboard"

func (t *TektonDashboard) GetName() string {
	return tektonDashboardName
}

func (t *TektonDashboard) DetailURL(ns, pr string) string {
	return fmt.Sprintf("%s/#/namespaces/%s/pipelineruns/%s", t.BaseURL, ns, pr)
}

func (t *TektonDashboard) TaskLogURL(ns, pr, task string) string {
	return fmt.Sprintf("%s?pipelineTask=%s", t.DetailURL(ns, pr), task)
}

func (t *TektonDashboard) URL() string {
	return t.BaseURL
}

func (t *TektonDashboard) UI(ctx context.Context, kdyn dynamic.Interface) error {
	return nil
}
