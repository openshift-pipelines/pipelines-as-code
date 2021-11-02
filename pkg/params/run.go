package params

import (
	"context"
	"os"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/consoleui"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Run struct {
	Clients clients.Clients
	Info    info.Info
}

// GetConfigFromConfigMap get config from configmap, we should remove all the
// logics from cobra flags and just support configmap config and env config in the future.
func (r *Run) GetConfigFromConfigMap(ctx context.Context) error {
	cfg, err := r.Clients.Kube.CoreV1().ConfigMaps(info.PACInstallNS).Get(ctx, info.PACConfigmapNS, v1.GetOptions{})
	if err != nil {
		return err
	}

	if r.Info.Pac.ApplicationName == "" {
		if applicationName, ok := cfg.Data["application-name"]; ok {
			r.Info.Pac.ApplicationName = applicationName
		} else {
			r.Info.Pac.ApplicationName = info.PACApplicationName
		}
	}

	if secretAutoCreation, ok := cfg.Data["secret-auto-create"]; ok {
		sv := false

		if strings.ToLower(secretAutoCreation) == "true" ||
			strings.ToLower(secretAutoCreation) == "yes" || secretAutoCreation == "1" {
			sv = true
		}
		r.Info.Pac.SecretAutoCreation = sv
	}

	if tektonDashboardURL, ok := cfg.Data["tekton-dashboard-url"]; ok {
		r.Clients.Log.Infof("using tekton dashboard url on: %s", tektonDashboardURL)
		r.Clients.ConsoleUI = &consoleui.TektonDashboard{BaseURL: tektonDashboardURL}
	}
	if os.Getenv("PAC_TEKTON_DASHBOARD_URL") != "" {
		r.Clients.Log.Infof("using tekton dashboard url on: %s", os.Getenv("PAC_TEKTON_DASHBOARD_URL"))
		r.Clients.ConsoleUI = &consoleui.TektonDashboard{BaseURL: os.Getenv("PAC_TEKTON_DASHBOARD_URL")}
	}
	return nil
}

func New() *Run {
	return &Run{
		Info: info.Info{
			Event: &info.Event{},
			Pac:   &info.PacOpts{},
		},
	}
}
