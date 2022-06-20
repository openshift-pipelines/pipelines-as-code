package setup

import (
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/spf13/cobra"
)

func bitbucketCloudWebhookCommand(run *params.Run, ioStreams *cli.IOStreams) *cobra.Command {
	return buildProviderCommand(run, ioStreams, "bitbucket-cloud")
}
