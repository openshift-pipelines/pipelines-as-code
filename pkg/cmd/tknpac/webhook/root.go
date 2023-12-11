package webhook

import (
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/spf13/cobra"
)

func Root(clients *params.Run, ioStreams *cli.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "webhook",
		Aliases:      []string{},
		Short:        "Update webhook secret",
		Long:         `Update webhook secret with token and personal access token`,
		SilenceUsage: true,
		Annotations: map[string]string{
			"commandType": "main",
		},
	}

	cmd.AddCommand(webhookAdd(clients, ioStreams))
	cmd.AddCommand(webhookUpdateToken(clients, ioStreams))
	return cmd
}
