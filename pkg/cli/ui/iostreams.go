package ui

import (
	"fmt"
	"io"
	"os"
	"time"

	surveyCore "github.com/AlecAivazis/survey/v2/core"
	"github.com/briandowns/spinner"
	"github.com/mattn/go-colorable"
	"github.com/mattn/go-isatty"
	"github.com/mgutz/ansi"
)

type IOStreams struct {
	In     io.ReadCloser
	Out    io.Writer
	ErrOut io.Writer

	progressIndicator        *spinner.Spinner
	colorEnabled             bool
	progressIndicatorEnabled bool
	stdoutTTYOverride        bool
	stderrTTYOverride        bool
	stderrIsTTY              bool
	stdoutIsTTY              bool
	is256enabled             bool
}

func (s *IOStreams) StartProgressIndicator() {
	sp := spinner.New(spinner.CharSets[11], 400*time.Millisecond, spinner.WithWriter(s.ErrOut))
	sp.Start()
	s.progressIndicator = sp
}

func (s *IOStreams) StopProgressIndicator() {
	if s.progressIndicator == nil {
		return
	}
	s.progressIndicator.Stop()
	s.progressIndicator = nil
}

func (s *IOStreams) ColorScheme() *ColorScheme {
	return NewColorScheme(s.ColorEnabled(), s.ColorSupport256())
}

func (s *IOStreams) ColorEnabled() bool {
	return s.colorEnabled
}

func (s *IOStreams) SetColorEnabled(colorEnabled bool) {
	s.colorEnabled = colorEnabled
	s.setSurveyColor()
	s.progressIndicatorEnabled = colorEnabled
}

func (s *IOStreams) setSurveyColor() {
	if !s.colorEnabled {
		surveyCore.DisableColor = true
	} else {
		// override survey's poor choice of color
		surveyCore.TemplateFuncsWithColor["color"] = func(style string) string {
			switch style {
			case "white":
				if s.ColorSupport256() {
					return fmt.Sprintf("\x1b[%d;5;%dm", 38, 242)
				}
				return ansi.ColorCode("default")
			default:
				return ansi.ColorCode(style)
			}
		}
	}
}

func (s *IOStreams) ColorSupport256() bool {
	return s.is256enabled
}

func NewIOStreams() *IOStreams {
	stdoutIsTTY := isTerminal(os.Stdout)
	stderrIsTTY := isTerminal(os.Stderr)

	io := &IOStreams{
		In:           os.Stdin,
		Out:          colorable.NewColorable(os.Stdout),
		ErrOut:       colorable.NewColorable(os.Stderr),
		colorEnabled: EnvColorForced() || (!EnvColorDisabled() && stdoutIsTTY),
		is256enabled: Is256ColorSupported(),
	}

	if stdoutIsTTY && stderrIsTTY {
		io.progressIndicatorEnabled = true
	}

	io.setSurveyColor()

	// prevent duplicate isTerminal queries now that we know the answer
	io.SetStdoutTTY(stdoutIsTTY)
	io.SetStderrTTY(stderrIsTTY)
	return io
}

func (s *IOStreams) SetStdoutTTY(isTTY bool) {
	s.stdoutTTYOverride = true
	s.stdoutIsTTY = isTTY
}

func (s *IOStreams) SetStderrTTY(isTTY bool) {
	s.stderrTTYOverride = true
	s.stderrIsTTY = isTTY
}

func (s *IOStreams) IsStdoutTTY() bool {
	if s.stdoutTTYOverride {
		return s.stdoutIsTTY
	}
	if stdout, ok := s.Out.(*os.File); ok {
		return isTerminal(stdout)
	}
	return false
}

func isTerminal(f *os.File) bool {
	return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
}
