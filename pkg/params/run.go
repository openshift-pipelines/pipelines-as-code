package params

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/consoleui"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
)

const (
	PACConfigmapName        = "pipelines-as-code"
	StartingPipelineRunText = `Starting Pipelinerun <b>%s</b> in namespace
  <b>%s</b><br><br>You can monitor the execution using the [%s](%s) PipelineRun viewer or through the command line by using the following command:
    <br><code>%s pr logs -n %s %s -f</code>`
	QueuingPipelineRunText = `PipelineRun <b>%s</b> has been queued in namespace
  <b>%s</b><br><br>`
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

// WatchConfigMapChanges watches for provide configmap
func (r *Run) WatchConfigMapChanges(ctx context.Context) error {
	ns := os.Getenv("SYSTEM_NAMESPACE")
	if ns == "" {
		return fmt.Errorf("failed to find pipelines-as-code installation namespace")
	}
	watcher, err := r.Clients.Kube.CoreV1().ConfigMaps(ns).Watch(ctx, v1.SingleObject(v1.ObjectMeta{
		Name:      PACConfigmapName,
		Namespace: ns,
	}))
	if err != nil {
		return fmt.Errorf("unable to create watcher : %w", err)
	}
	if err := r.getConfigFromConfigMapWatcher(ctx, watcher.ResultChan()); err != nil {
		return fmt.Errorf("failed to get defaults : %w", err)
	}
	return nil
}

// getConfigFromConfigMapWatcher get config from configmap, we should remove all the
// logics from cobra flags and just support configmap config and environment config in the future.
func (r *Run) getConfigFromConfigMapWatcher(ctx context.Context, eventChannel <-chan watch.Event) error {
	for {
		event, open := <-eventChannel
		if open {
			switch event.Type {
			case watch.Added, watch.Modified:
				if err := r.UpdatePACInfo(ctx); err != nil {
					r.Clients.Log.Info("failed to update PAC info", err)
					return err
				}
			case watch.Deleted, watch.Bookmark, watch.Error:
				// added this case block to avoid lint issues
				// Do nothing
			default:
				// Do nothing
			}
		} else {
			// If eventChannel is closed, it means the server has closed the connection
			return nil
		}
	}
}

func (r *Run) UpdatePACInfo(ctx context.Context) error {
	ns := os.Getenv("SYSTEM_NAMESPACE")
	if ns == "" {
		return fmt.Errorf("failed to find pipelines-as-code installation namespace")
	}
	// TODO: move this to kubeinteractions class so we can add unittests.
	cfg, err := r.Clients.Kube.CoreV1().ConfigMaps(ns).Get(ctx, PACConfigmapName, v1.GetOptions{})
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
		},
	}
}
