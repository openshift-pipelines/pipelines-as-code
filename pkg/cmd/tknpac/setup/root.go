package setup

import (
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
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
	return cmd
}
