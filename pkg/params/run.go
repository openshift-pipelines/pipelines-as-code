package params

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/consoleui"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Run struct {
	Clients clients.Clients
	Info    info.Info
}

func (r *Run) UpdatePACInfo(ctx context.Context) error {
	ns := info.GetNS(ctx)
	if ns == "" {
		return fmt.Errorf("failed to find namespace")
	}

	// TODO: move this to kubeinteractions class so we can add unittests.
	cfg, err := r.Clients.Kube.CoreV1().ConfigMaps(ns).Get(ctx, r.Info.Controller.Configmap, metav1.GetOptions{})
	if err != nil {
		return err
	}

	updatedSettings, err := r.Info.UpdatePacOpts(r.Clients.Log, cfg.Data)
	if err != nil {
		return err
	}

	if updatedSettings.TektonDashboardURL != "" && updatedSettings.TektonDashboardURL != r.Clients.ConsoleUI().URL() {
		r.Clients.Log.Infof("updating console url to: %s", updatedSettings.TektonDashboardURL)
		r.Clients.SetConsoleUI(&consoleui.TektonDashboard{BaseURL: updatedSettings.TektonDashboardURL})
	}
	if os.Getenv("PAC_TEKTON_DASHBOARD_URL") != "" {
		r.Clients.Log.Infof("using tekton dashboard url on: %s", os.Getenv("PAC_TEKTON_DASHBOARD_URL"))
		r.Clients.SetConsoleUI(&consoleui.TektonDashboard{BaseURL: os.Getenv("PAC_TEKTON_DASHBOARD_URL")})
	}
	if updatedSettings.CustomConsoleURL != "" {
		r.Clients.Log.Infof("updating console url to: %s", updatedSettings.CustomConsoleURL)
		r.Clients.SetConsoleUI(&consoleui.CustomConsole{})
	}

	// This is the case when reverted settings for CustomConsole and TektonDashboard then URL should point to OpenshiftConsole for Openshift platform
	if updatedSettings.CustomConsoleURL == "" &&
		(updatedSettings.TektonDashboardURL == "" && os.Getenv("PAC_TEKTON_DASHBOARD_URL") == "") {
		r.Clients.SetConsoleUI(&consoleui.OpenshiftConsole{})
		_ = r.Clients.ConsoleUI().UI(ctx, r.Clients.Dynamic)
	}

	return nil
}

func New() *Run {
	hubCatalog := &sync.Map{}
	hubCatalog.Store("default", settings.HubCatalog{
		ID:   "default",
		Name: settings.HubCatalogNameDefaultValue,
		URL:  settings.HubURLDefaultValue,
	})
	return &Run{
		Info: info.Info{
			Pac: &info.PacOpts{
				Settings: settings.Settings{
					ApplicationName: settings.PACApplicationNameDefaultValue,
					HubCatalogs:     hubCatalog,
				},
			},
			Kube:       &info.KubeOpts{},
			Controller: &info.ControllerInfo{},
		},
	}
}
