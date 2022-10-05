package params

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/consoleui"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
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
func (r *Run) WatchConfigMapChanges(ctx context.Context, run *Run) error {
	ns := os.Getenv("SYSTEM_NAMESPACE")
	if ns == "" {
		return fmt.Errorf("failed to find pipelines-as-code installation namespace")
	}
	watcher, err := r.Clients.Kube.CoreV1().ConfigMaps(ns).Watch(ctx, v1.SingleObject(v1.ObjectMeta{
		Name:      info.PACConfigmapName,
		Namespace: ns,
	}))
	if err != nil {
		return fmt.Errorf("unable to create watcher : %w", err)
	}
	if err := run.getConfigFromConfigMapWatcher(ctx, watcher.ResultChan()); err != nil {
		return fmt.Errorf("failed to get defaults : %w", err)
	}
	return nil
}

// getConfigFromConfigMapWatcher get config from configmap, we should remove all the
// logics from cobra flags and just support configmap config and env config in the future.
func (r *Run) getConfigFromConfigMapWatcher(ctx context.Context, eventChannel <-chan watch.Event) error {
	for {
		event, open := <-eventChannel
		if open {
			switch event.Type {
			case watch.Added, watch.Modified:
				if err := r.UpdatePACInfo(ctx); err != nil {
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
	cfg, err := r.Clients.Kube.CoreV1().ConfigMaps(ns).Get(ctx, info.PACConfigmapName, v1.GetOptions{})
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

	if hubCatalogName, ok := cfg.Data["hub-catalog-name"]; ok {
		r.Info.Pac.HubCatalogName = hubCatalogName
	} else {
		r.Info.Pac.HubCatalogName = info.HubCatalogName
	}

	if remoteTask, ok := cfg.Data["remote-tasks"]; ok {
		r.Info.Pac.RemoteTasks = StringToBool(remoteTask)
	}

	if check, ok := cfg.Data["bitbucket-cloud-check-source-ip"]; ok {
		r.Info.Pac.BitbucketCloudCheckSourceIP = StringToBool(check)
	}

	if sourceIP, ok := cfg.Data["bitbucket-cloud-additional-source-ip"]; ok {
		r.Info.Pac.BitbucketCloudAdditionalSourceIP = sourceIP
	}

	return nil
}

func New() *Run {
	return &Run{
		Info: info.Info{
			Pac: &info.PacOpts{
				ApplicationName: info.PACApplicationName,
				HubURL:          info.HubURL,
			},
		},
	}
}
