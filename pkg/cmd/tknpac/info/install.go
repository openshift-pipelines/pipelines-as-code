package info

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"
	"text/template"

	"github.com/google/go-github/v56/github"
	"github.com/juju/ansiterm"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/github/app"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type InstallInfo struct {
	App        *github.App
	run        *params.Run
	jwtToken   string
	apiURL     string
	HookConfig *github.HookConfig
}

//go:embed templates/info.tmpl
var infoTemplate string

func (g *InstallInfo) hookConfig(ctx context.Context) error {
	resp, err := app.GetReponse(ctx, "GET", fmt.Sprintf("%s/app/hook/config", g.apiURL), g.jwtToken, g.run)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	// parse json response
	return json.Unmarshal(data, &g.HookConfig)
}

func (g *InstallInfo) get(ctx context.Context) error {
	resp, err := app.GetReponse(ctx, "GET", fmt.Sprintf("%s/app", g.apiURL), g.jwtToken, g.run)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	// parse json response
	return json.Unmarshal(data, &g.App)
}

func install(ctx context.Context, run *params.Run, ios *cli.IOStreams, apiURL string) error {
	targetNs, version, err := params.GetInstallLocation(ctx, run)
	if err != nil {
		return err
	}
	info := &InstallInfo{run: run, apiURL: apiURL}
	if jwtToken, err := app.GenerateJWT(ctx, targetNs, info.run); err == nil {
		info.jwtToken = jwtToken
		if err := info.get(ctx); err != nil {
			return err
		}
		if err := info.hookConfig(ctx); err != nil {
			return err
		}
	}
	repos, err := run.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("cannot list all repo on cluster, check your rights and that paac is installed: %w", err)
	}
	reposItems := &repos.Items
	args := struct {
		Info             *InstallInfo
		InstallNamespace string
		Version          string
		Repos            *[]v1alpha1.Repository
		CS               *cli.ColorScheme
	}{
		Info:             info,
		InstallNamespace: targetNs,
		Version:          version,
		Repos:            reposItems,
		CS:               ios.ColorScheme(),
	}
	w := ansiterm.NewTabWriter(ios.Out, 0, 5, 3, ' ', tabwriter.TabIndent)
	t := template.Must(template.New("Describe Repository").Parse(infoTemplate))
	if err := t.Execute(w, args); err != nil {
		return err
	}

	return w.Flush()
}

func installCommand(run *params.Run, ioStreams *cli.IOStreams) *cobra.Command {
	var apiURL string
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Provides installation info for pipelines-as-code (admin only).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()
			if err := run.Clients.NewClients(ctx, &run.Info); err != nil {
				return err
			}
			return install(ctx, run, ioStreams, apiURL)
		},
		Annotations: map[string]string{
			"commandType": "main",
		},
	}
	// add params for enteprise github
	cmd.PersistentFlags().StringVarP(&apiURL, "github-api-url", "", "https://api.github.com", "Github API URL")
	return cmd
}
