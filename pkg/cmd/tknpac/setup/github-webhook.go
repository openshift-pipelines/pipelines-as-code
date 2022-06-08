package setup

import (
	"os"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli/webhook"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/git"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/spf13/cobra"
)

func githubWebhookCommand(run *params.Run, ioStreams *cli.IOStreams) *cobra.Command {
	var githubURLForWebhook, pacNamespace string
	cmd := &cobra.Command{
		Use:     "github-webhook",
		Aliases: []string{""},
		Short:   "Setup GitHub Webhook with Pipelines As Code",
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
				ProviderAPIURL: githubURLForWebhook,
				IOStreams:      ioStreams,
			}

			return config.Install(ctx, webhook.ProviderTypeGitHub)
		},
		Annotations: map[string]string{
			"commandType": "main",
		},
	}
	cmd.PersistentFlags().StringVarP(&pacNamespace, "pac-namespace", "", "", "The namespace where pac is installed")
	cmd.PersistentFlags().StringVarP(&githubURLForWebhook, "github-api-url", "", "", "GitHub Enterprise API URL")
	return cmd
}
