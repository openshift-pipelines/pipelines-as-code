package repository

import (
	"regexp"
	"testing"

	"github.com/AlecAivazis/survey/v2/terminal"
	goexpect "github.com/Netflix/go-expect"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/git"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/ui"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/prompt"
	"gotest.tools/v3/assert"
)

func TestCreate(t *testing.T) {
	tests := []struct {
		name         string
		opts         *createOptions
		prompt       prompt.Prompt
		expectURL    string
		expectErrStr string
	}{
		{
			name:      "Creating Repository URL with Git info URL",
			expectURL: "http://tartonpion",
			opts: &createOptions{
				event:   &info.Event{},
				gitInfo: &git.Info{URL: "http://tartonpion"},
				cliOpts: &params.PacCliOpts{},
			},
			prompt: prompt.Prompt{
				CmdArgs: []string{},
				Procedure: func(c *goexpect.Console) error {
					reg := regexp.MustCompile("Enter the Git repository url containing the pipelines.*http://tartonpion")
					if _, err := c.Expect(goexpect.Regexp(reg)); err != nil {
						return err
					}

					if _, err := c.SendLine(string(terminal.KeyEnter)); err != nil {
						return err
					}
					if _, err := c.SendLine(string(terminal.KeyEnter)); err != nil {
						return err
					}
					return nil
				},
			},
		},
		{
			name:         "Creating Repository URL without Git info URL",
			expectErrStr: "no string has been provided",
			opts: &createOptions{
				event:   &info.Event{},
				gitInfo: &git.Info{},
				cliOpts: &params.PacCliOpts{},
			},
			prompt: prompt.Prompt{
				CmdArgs: []string{},
				Procedure: func(c *goexpect.Console) error {
					reg := regexp.MustCompile("Enter the Git repository url containing the pipelines")
					if _, err := c.Expect(goexpect.Regexp(reg)); err != nil {
						return err
					}

					if _, err := c.SendLine(string(terminal.KeyEnter)); err != nil {
						return err
					}
					if _, err := c.SendLine(string(terminal.KeyEnter)); err != nil {
						return err
					}
					return nil
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.prompt.RunTest(t, tt.prompt.Procedure, func(stdio terminal.Stdio) error {
				tt.opts.cliOpts.AskOpts = prompt.WithStdio(stdio)
				tt.opts.ioStreams = &ui.IOStreams{Out: stdio.Out, ErrOut: stdio.Err}
				return getRepoURL(tt.opts)
			})
			if tt.expectErrStr != "" {
				assert.ErrorContains(t, err, tt.expectErrStr)
			}
			assert.Equal(t, tt.expectURL, tt.opts.event.URL)
		})
	}
}
