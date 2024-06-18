package auth

import (
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/spf13/cobra"
)

func Root(clients *params.Run, ioStreams *cli.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "auth",
		Aliases:      []string{},
		Short:        "Authenticate user",
		Long:         `Authenticate users with git provider`,
		SilenceUsage: true,
		Annotations: map[string]string{
			"commandType": "main",
		},
	}

	cmd.AddCommand(loginCommand(clients, ioStreams))
	return cmd
}
