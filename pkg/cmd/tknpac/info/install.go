package info

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"
	"text/template"

	_ "embed"

	"github.com/google/go-github/v50/github"
	"github.com/juju/ansiterm"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/github/app"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var targetNamespaces = []string{"openshift-pipelines", "pipelines-as-code"}

// google/go-github is missing the count from their struct
type GithubApp struct {
	*github.App
	InstallationsCount int `json:"installations_count,omitempty"`
}

//go:embed templates/info.tmpl
var infoTemplate string

func getPacLocation(ctx context.Context, run *params.Run) (string, string, error) {
	for _, ns := range targetNamespaces {
		version := "unknown"
		deployment, err := run.Clients.Kube.AppsV1().Deployments(ns).Get(ctx, "pipelines-as-code-controller", metav1.GetOptions{})
		if err != nil {
			continue
		}
		if val, ok := deployment.GetLabels()["app.kubernetes.io/version"]; ok {
			version = val
		}
		return ns, version, nil
	}
	return "", "", fmt.Errorf("cannot find your pipelines-as-code installation, check that it is installed and you have access")
}

func install(ctx context.Context, run *params.Run, ios *cli.IOStreams, apiURL string) error {
	targetNs, version, err := getPacLocation(ctx, run)
	if err != nil {
		return err
	}
	var gapp *GithubApp
	jwtToken, err := app.GenerateJWT(ctx, targetNs, run)
	if err == nil {
		resp, err := app.GetReponse(ctx, "GET", fmt.Sprintf("%s/app", apiURL), jwtToken, run)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		// parse json response
		if err := json.Unmarshal(data, &gapp); err != nil {
			return err
		}
	}
	repos, err := run.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("cannot list alll repo on cluster, check your rights and that paac is installed: %w", err)
	}
	reposItems := &repos.Items
	args := struct {
		Gapp             *GithubApp
		InstallNamespace string
		Version          string
		Repos            *[]v1alpha1.Repository
		CS               *cli.ColorScheme
	}{
		Gapp:             gapp,
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
		RunE: func(cmd *cobra.Command, args []string) error {
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
