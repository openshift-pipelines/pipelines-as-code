package cli

import (
	"bytes"

	"github.com/spf13/cobra"
)

// ExecuteCommand executes the root command passing the args and returns
// the output as a string and error.
func ExecuteCommand(root *cobra.Command, args ...string) (string, error) {
	_, output, err := ExecuteCommandC(root, args...)
	return output, err
}

// ExecuteCommandC executes the root command passing the args and returns
// the root command, output as a string and error if any.
func ExecuteCommandC(c *cobra.Command, args ...string) (*cobra.Command, string, error) {
	buf := new(bytes.Buffer)
	c.SetOutput(buf)
	c.SetArgs(args)
	c.SilenceUsage = true

	root, err := c.ExecuteC()

	return root, buf.String(), err
}
