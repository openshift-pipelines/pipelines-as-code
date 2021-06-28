package repository

import (
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/spf13/cobra"
)

func Root(p cli.Params) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "repository",
		Aliases:      []string{"repo", "repsitories"},
		Short:        "Pipeline as Code repositories",
		Long:         `Manage Pipeline as Code repositories`,
		SilenceUsage: true,
	}

	cmd.AddCommand(ListCommand(p))
	cmd.AddCommand(DescribeCommand(p))
	cmd.AddCommand(CreateCommand(p))

	return cmd
}
