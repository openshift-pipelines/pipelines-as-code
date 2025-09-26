package tknpac

import (
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/tknpac/bootstrap"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/tknpac/cel"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/tknpac/completion"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/tknpac/create"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/tknpac/deleterepo"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/tknpac/describe"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/tknpac/generate"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/tknpac/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/tknpac/list"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/tknpac/logs"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/tknpac/resolve"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/tknpac/version"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/tknpac/webhook"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/spf13/cobra"
)

func Root(clients *params.Run) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "tkn-pac",
		Short:        "Pipelines-as-Code CLI",
		Long:         `tkn plugin to use Pipelines-as-Code as a CLI`,
		SilenceUsage: true,
		Annotations: map[string]string{
			"commandType": "main",
		},
	}
	clients.Info.Kube.AddFlags(cmd)

	ioStreams := cli.NewIOStreams()

	cmd.AddCommand(version.Command(ioStreams))
	cmd.AddCommand(info.Root(clients, ioStreams))
	cmd.AddCommand(create.Root(clients, ioStreams))
	cmd.AddCommand(list.Root(clients, ioStreams))
	cmd.AddCommand(deleterepo.Root(clients, ioStreams))
	cmd.AddCommand(describe.Root(clients, ioStreams))
	cmd.AddCommand(logs.Command(clients, ioStreams))
	cmd.AddCommand(resolve.Command(clients, ioStreams))
	cmd.AddCommand(completion.Command())
	cmd.AddCommand(bootstrap.Command(clients, ioStreams))
	cmd.AddCommand(generate.Command(clients, ioStreams))
	cmd.AddCommand(cel.Command(ioStreams))
	cmd.AddCommand(webhook.Root(clients, ioStreams))
	return cmd
}
