package cli

import (
	"bytes"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	cli2 "github.com/openshift-pipelines/pipelines-as-code/pkg/test/cli"
	"github.com/spf13/cobra"
)

func ExecCommand(runcnx *params.Run, cmd func(*params.Run, *cli.IOStreams) *cobra.Command, args ...string) (string, error) {
	bufout := new(bytes.Buffer)
	ecmd := cmd(runcnx, &cli.IOStreams{
		Out: bufout,
	})
	_, err := cli2.ExecuteCommand(ecmd, args...)
	return bufout.String(), err
}
