package flags

import (
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
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
	AskOpts       survey.AskOpt
}

func NewCliOptions(cmd *cobra.Command) (*CliOpts, error) {
	var err error
	c := &CliOpts{
		AskOpts: func(opt *survey.AskOptions) error {
			opt.Stdio = terminal.Stdio{
				In:  os.Stdin,
				Out: os.Stdout,
				Err: os.Stderr,
			}
			return nil
		},
	}
	c.NoColoring, err = cmd.Flags().GetBool(noColor)
	if err != nil {
		return nil, err
	}
	c.Namespace, err = cmd.Flags().GetString(namespace)
	if err != nil {
		return nil, err
	}
	return c, err
}

func (c *CliOpts) Ask(resource string, options []string) (string, error) {
	var ans string
	qs := []*survey.Question{
		{
			Name: resource,
			Prompt: &survey.Select{
				Message: fmt.Sprintf("Select a %s", resource),
				Options: options,
			},
		},
	}

	if err := survey.Ask(qs, &ans, c.AskOpts); err != nil {
		return "", err
	}
	return ans, nil
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
