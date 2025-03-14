package params

import (
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
)

type PacCliOpts struct {
	NoColoring    bool
	AllNameSpaces bool
	Namespace     string
	AskOpts       survey.AskOpt
}

func NewCliOptions() *PacCliOpts {
	return &PacCliOpts{
		AskOpts: func(opt *survey.AskOptions) error {
			opt.Stdio = terminal.Stdio{
				In:  os.Stdin,
				Out: os.Stdout,
				Err: os.Stderr,
			}
			return nil
		},
	}
}

func (c *PacCliOpts) Ask(qss []*survey.Question, answer any) error {
	return survey.Ask(qss, answer, c.AskOpts)
}
