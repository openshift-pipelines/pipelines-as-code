package repository

import (
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli/ui"
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
	ioStreams := ui.NewIOStreams()

	cmd.AddCommand(ListCommand(p, ioStreams))
	cmd.AddCommand(DescribeCommand(p, ioStreams))
	cmd.AddCommand(CreateCommand(p, ioStreams))

	return cmd
}
