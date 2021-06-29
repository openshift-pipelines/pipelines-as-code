package main

import (
	"os"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/completion"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/repository"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/resolve"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/flags"
	"github.com/spf13/cobra"
)

func Root(p cli.Params) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "tkn-pac",
		Short:        "Pipeline as Code CLI",
		Long:         `This is the Pipelines as Code`,
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return flags.InitParams(p, cmd)
		},
	}
	flags.AddPacOptions(cmd)

	cmd.AddCommand(repository.Root(p))
	cmd.AddCommand(resolve.Command(p))
	cmd.AddCommand(completion.Command())

	return cmd
}

func main() {
	tp := &cli.PacParams{}
	pac := Root(tp)

	if err := pac.Execute(); err != nil {
		os.Exit(1)
	}
}
