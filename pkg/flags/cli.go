package flags

import (
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli/completion"
	"github.com/spf13/cobra"
)

var (
	noColor   = "no-color"
	namespace = "namespace"
)

type CliOpts struct {
	NoColoring    bool
	AllNameSpaces bool
	Namespace     string
}

func (c *CliOpts) SetFromFlags(cmd *cobra.Command) error {
	var err error
	c.NoColoring, err = cmd.Flags().GetBool(noColor)
	if err != nil {
		return err
	}
	c.Namespace, err = cmd.Flags().GetString(namespace)
	if err != nil {
		return err
	}
	return err
}

func AddPacCliOptions(cmd *cobra.Command) {
	cmd.PersistentFlags().StringP(
		namespace, "n", "",
		"If present, the namespace scope for this CLI request")

	_ = cmd.RegisterFlagCompletionFunc(namespace,
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return completion.BaseCompletion(namespace, args)
		},
	)

	cmd.PersistentFlags().BoolP(noColor, "C", false, "disable coloring")
}
