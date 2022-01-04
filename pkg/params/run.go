package params

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/consoleui"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Run struct {
	Clients clients.Clients
	Info    info.Info
}

func StringToBool(s string) bool {
	if strings.ToLower(s) == "true" ||
		strings.ToLower(s) == "yes" || s == "1" {
		return true
	}
	return false
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
		r.Info.Pac.SecretAutoCreation = StringToBool(secretAutoCreation)
	}

	if tektonDashboardURL, ok := cfg.Data["tekton-dashboard-url"]; ok {
		r.Clients.Log.Infof("using tekton dashboard url on: %s", tektonDashboardURL)
		r.Clients.ConsoleUI = &consoleui.TektonDashboard{BaseURL: tektonDashboardURL}
	}
	if os.Getenv("PAC_TEKTON_DASHBOARD_URL") != "" {
		r.Clients.Log.Infof("using tekton dashboard url on: %s", os.Getenv("PAC_TEKTON_DASHBOARD_URL"))
		r.Clients.ConsoleUI = &consoleui.TektonDashboard{BaseURL: os.Getenv("PAC_TEKTON_DASHBOARD_URL")}
	}

	if hubURL, ok := cfg.Data["hub-url"]; ok {
		r.Info.Pac.HubURL = hubURL
	} else {
		r.Info.Pac.HubURL = info.HubURL
	}

	if remoteTask, ok := cfg.Data["remote-tasks"]; ok {
		r.Info.Pac.RemoteTasks = StringToBool(remoteTask)
	}

	if timeout, ok := cfg.Data["default-pipelinerun-timeout"]; ok {
		parsedTimeout, err := time.ParseDuration(timeout)
		if err != nil {
			r.Clients.Log.Infof("failed to parse default-pipelinerun-timeout: %s, using %v as default timeout",
				cfg.Data["default-pipelinerun-timeout"], info.DefaultPipelineRunTimeout)
			r.Info.Pac.DefaultPipelineRunTimeout = info.DefaultPipelineRunTimeout
		} else {
			r.Info.Pac.DefaultPipelineRunTimeout = parsedTimeout
		}
	} else {
		r.Info.Pac.DefaultPipelineRunTimeout = info.DefaultPipelineRunTimeout
	}

	return nil
}

func New() *Run {
	return &Run{
		Info: info.Info{
			Event: &info.Event{},
			Pac: &info.PacOpts{
				ApplicationName: info.PACApplicationName,
				HubURL:          info.HubURL,
			},
		},
	}
}
