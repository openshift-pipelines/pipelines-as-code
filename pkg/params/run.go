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

const (
	PACConfigmapName = "pipelines-as-code"
)

type Run struct {
	Clients clients.Clients
	Info    info.Info
}

var mutex = &sync.Mutex{}

func (r *Run) UpdatePACInfo(ctx context.Context) error {
	mutex.Lock()
	defer mutex.Unlock()
	ns := info.GetNS(ctx)
	if ns == "" {
		return fmt.Errorf("failed to find namespace")
	}

	// TODO: move this to kubeinteractions class so we can add unittests.
	cfg, err := r.Clients.Kube.CoreV1().ConfigMaps(ns).Get(ctx, r.Info.Controller.Configmap, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if err = settings.ConfigToSettings(r.Clients.Log, r.Info.Pac.Settings, cfg.Data); err != nil {
		return err
	}

	if r.Info.Pac.Settings.TektonDashboardURL != "" && r.Info.Pac.Settings.TektonDashboardURL != r.Clients.ConsoleUI.URL() {
		r.Clients.Log.Infof("updating console url to: %s", r.Info.Pac.Settings.TektonDashboardURL)
		r.Clients.ConsoleUI = &consoleui.TektonDashboard{BaseURL: r.Info.Pac.Settings.TektonDashboardURL}
	}
	if os.Getenv("PAC_TEKTON_DASHBOARD_URL") != "" {
		r.Clients.Log.Infof("using tekton dashboard url on: %s", os.Getenv("PAC_TEKTON_DASHBOARD_URL"))
		r.Clients.ConsoleUI = &consoleui.TektonDashboard{BaseURL: os.Getenv("PAC_TEKTON_DASHBOARD_URL")}
	}
	if r.Info.Pac.Settings.CustomConsoleURL != "" {
		r.Clients.Log.Infof("updating console url to: %s", r.Info.Pac.Settings.CustomConsoleURL)
		r.Clients.ConsoleUI = &consoleui.CustomConsole{Info: &r.Info}
	}

	// This is the case when reverted settings for CustomConsole and TektonDashboard then URL should point to OpenshiftConsole for Openshift platform
	if r.Info.Pac.Settings.CustomConsoleURL == "" &&
		(r.Info.Pac.Settings.TektonDashboardURL == "" && os.Getenv("PAC_TEKTON_DASHBOARD_URL") == "") {
		r.Clients.ConsoleUI = &consoleui.OpenshiftConsole{}
		_ = r.Clients.ConsoleUI.UI(ctx, r.Clients.Dynamic)
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
				Settings: &settings.Settings{
					ApplicationName: settings.PACApplicationNameDefaultValue,
					HubCatalogs:     hubCatalog,
				},
			},
			Kube:       &info.KubeOpts{},
			Controller: &info.ControllerInfo{},
		},
	}
}
