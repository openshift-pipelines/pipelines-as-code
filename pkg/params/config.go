package params

import (
	"context"
	"fmt"
	"os"

	"github.com/fsnotify/fsnotify"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/consoleui"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
	"github.com/spf13/viper"
)

const (
	// mount path for configmap
	configPath = "/etc/pipelines-as-code/config"
	// name of field in configmap under which configurations are nested,
	// without extension
	configName = "config"
)

func (r *Run) InitConfig(ctx context.Context) error {
	r.configWatcher = viper.New()
	r.configWatcher.SetConfigName(configName)
	r.configWatcher.AddConfigPath(configPath)
	r.configWatcher.AutomaticEnv()

	err := r.configWatcher.ReadInConfig()
	if err != nil {
		r.Clients.Log.Fatalf("failed to read configurations: %w", err)
		return err
	}

	readSettings := func(ctx context.Context, r *Run) error {
		setting, err := settings.ReadConfig(r.configWatcher, r.Clients.Log)
		if err != nil {
			return fmt.Errorf("failed to read config into settings: %w", err)
		}

		r.Info.Pac = &info.PacOpts{
			Settings: setting,
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

	if err := readSettings(ctx, r); err != nil {
		return err
	}

	r.configWatcher.OnConfigChange(func(e fsnotify.Event) {
		// Don't fail if settings are invalid, log and emit event to let user know
		// continue with old settings
		if err := readSettings(ctx, r); err != nil {
			r.Clients.Log.Errorf("something went wrong reading config: %w", err)
		}
	})
	r.configWatcher.WatchConfig()
	return nil
}
