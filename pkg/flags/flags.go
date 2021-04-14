package flags

import (
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/spf13/cobra"
)

// InitParams initialises cli.Params based on flags defined in command
func InitParams(p cli.Params, cmd *cobra.Command) error {
	// ensure that the config is valid by creating a client
	if _, err := p.Clients(); err != nil {
		return err
	}
	return nil
}
