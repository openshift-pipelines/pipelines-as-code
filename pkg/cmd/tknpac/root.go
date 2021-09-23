package tknpac

import (
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/completion"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/repository"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/resolve"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/spf13/cobra"
)

func Root(clients *params.Run) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "tkn-pac",
		Short:        "Pipelines as Code CLI",
		Long:         `This is the the tkn plugin for Pipelines as Code CLI`,
		SilenceUsage: true,
	}
	clients.Info.Kube.AddFlags(cmd)

	cmd.AddCommand(repository.Root(clients))
	cmd.AddCommand(resolve.Command(clients))
	cmd.AddCommand(completion.Command())

	return cmd
}
