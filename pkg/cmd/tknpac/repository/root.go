package repository

import (
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/spf13/cobra"
)

func Root(clients *params.Run, ioStreams *cli.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "repository",
		Aliases:      []string{"repo", "repositories"},
		Short:        "Pipelines as Code repositories",
		Long:         `Manage Pipelines as Code repositories`,
		SilenceUsage: true,
		Annotations: map[string]string{
			"commandType": "main",
		},
	}
	cmd.AddCommand(DescribeCommand(clients, ioStreams))
	cmd.AddCommand(DeleteCommand(clients, ioStreams))

	return cmd
}
