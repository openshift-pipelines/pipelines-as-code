package params

import (
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/spf13/cobra"
)

var noColorFlag = "no-color"

type PacCliOpts struct {
	NoColoring    bool
	AllNameSpaces bool
	Namespace     string
	AskOpts       survey.AskOpt
}

func NewCliOptions(cmd *cobra.Command) (*PacCliOpts, error) {
	var err error
	c := &PacCliOpts{
		AskOpts: func(opt *survey.AskOptions) error {
			opt.Stdio = terminal.Stdio{
				In:  os.Stdin,
				Out: os.Stdout,
				Err: os.Stderr,
			}
			return nil
		},
	}
	c.NoColoring, err = cmd.Flags().GetBool(noColorFlag)
	if err != nil {
		return nil, err
	}
	return c, err
}

func (c *PacCliOpts) Ask(qss []*survey.Question, ans interface{}) error {
	return survey.Ask(qss, ans, c.AskOpts)
}
