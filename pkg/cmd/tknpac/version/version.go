package version

import (
	"fmt"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/version"
	"github.com/spf13/cobra"
)

func Command(ioStreams *cli.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: fmt.Sprintf("Print %s pac version", settings.TknBinaryName),
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Fprintln(ioStreams.Out, strings.TrimSpace(version.Version))
		},
		Annotations: map[string]string{
			"commandType": "main",
		},
	}
	return cmd
}
