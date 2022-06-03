package create

import (
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/spf13/cobra"
)

func Root(clients *params.Run, ioStreams *cli.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "create",
		Aliases:      []string{},
		Short:        "Create Pipelines as Code resources",
		Long:         `Create Pipelines as Code resources`,
		SilenceUsage: true,
		Annotations: map[string]string{
			"commandType": "main",
		},
	}

	cmd.AddCommand(repositoryCommand(clients, ioStreams))
	return cmd
}
