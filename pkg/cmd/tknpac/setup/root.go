package setup

import (
	"fmt"
	"os"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli/webhook"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/git"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/spf13/cobra"
)

func Root(clients *params.Run, ioStreams *cli.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "setup",
		Aliases:      []string{},
		Short:        "Setup provider app or webhook",
		Long:         `Setup provider app or webhook with pipelines as code`,
		SilenceUsage: true,
		Annotations: map[string]string{
			"commandType": "main",
		},
	}

	cmd.AddCommand(githubWebhookCommand(clients, ioStreams))
	cmd.AddCommand(gitlabWebhookCommand(clients, ioStreams))
	return cmd
}

func buildProviderCommand(run *params.Run, ioStreams *cli.IOStreams, provider string) *cobra.Command {
	var providerURL, pacNamespace string
	cmd := &cobra.Command{
		Use:     fmt.Sprintf("%s-webhook", provider),
		Aliases: []string{""},
		Short:   fmt.Sprintf("Setup %s Webhook with Pipelines As Code", strings.ToTitle(provider)),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			gitInfo := git.GetGitInfo(cwd)
			if err := run.Clients.NewClients(ctx, &run.Info); err != nil {
				return err
			}

			if err := run.Clients.NewClients(ctx, &run.Info); err != nil {
				return err
			}

			config := &webhook.Options{
				Run:            run,
				PACNamespace:   pacNamespace,
				RepositoryURL:  gitInfo.URL,
				ProviderAPIURL: providerURL,
				IOStreams:      ioStreams,
			}

			return config.Install(ctx, provider)
		},
		Annotations: map[string]string{
			"commandType": "main",
		},
	}
	cmd.PersistentFlags().StringVarP(&pacNamespace, "pac-namespace", "", "", "The namespace where pac is installed")
	cmd.PersistentFlags().StringVarP(&providerURL, fmt.Sprintf("%s-api-url", provider),
		"", "", fmt.Sprintf("%s Enterprise API URL", strings.ToTitle(provider)))

	return cmd
}
