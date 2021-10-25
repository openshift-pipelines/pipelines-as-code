package tknpac

import (
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/tknpac/bootstrap"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/tknpac/completion"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/tknpac/repository"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/tknpac/resolve"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/ui"
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

	ioStreams := ui.NewIOStreams()

	cmd.AddCommand(repository.Root(clients, ioStreams))
	cmd.AddCommand(resolve.Command(clients))
	cmd.AddCommand(completion.Command())
	cmd.AddCommand(bootstrap.Command(clients, ioStreams))
	return cmd
}
