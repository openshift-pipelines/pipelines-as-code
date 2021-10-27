package generate

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli/prompt"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/git"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/spf13/cobra"
)

var (
	eventTypes       = map[string]string{"pull_request": "Pull Request", "push": "Push to a Branch or a Tag"}
	defaultEventType = "Pull Request"
	mainBranch       = "main"
)

type Opts struct {
	event   *info.Event
	GitInfo *git.Info

	IOStreams *cli.IOStreams
	CLIOpts   *cli.PacCliOpts
}

func MakeOpts() *Opts {
	return &Opts{
		event:   &info.Event{},
		GitInfo: &git.Info{},

		IOStreams: &cli.IOStreams{},
		CLIOpts:   &cli.PacCliOpts{},
	}
}

func Command(ioStreams *cli.IOStreams) *cobra.Command {
	gopt := MakeOpts()
	gopt.IOStreams = ioStreams
	cmd := &cobra.Command{
		Use:     "generate",
		Aliases: []string{"gen"},
		Short:   "Generate PipelineRun",
		RunE: func(cmd *cobra.Command, args []string) error {
			gopt.CLIOpts = cli.NewCliOptions(cmd)
			gopt.IOStreams.SetColorEnabled(!gopt.CLIOpts.NoColoring)

			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			gopt.GitInfo = git.GetGitInfo(cwd)
			return Generate(gopt)
		},
	}
	return cmd
}

func Generate(o *Opts) error {
	if err := o.targetEvent(); err != nil {
		return err
	}

	if err := o.branchOrTag(); err != nil {
		return err
	}

	if err := o.samplePipeline(); err != nil {
		return err
	}
	return nil
}

func (o *Opts) targetEvent() error {
	var choice string

	msg := "Enter the Git event type for triggering the pipeline: "

	eventLabels := make([]string, 0, len(eventTypes))
	for _, label := range eventTypes {
		eventLabels = append(eventLabels, label)
	}
	if err := prompt.SurveyAskOne(
		&survey.Select{
			Message: msg,
			Options: eventLabels,
			Default: 0,
		}, &choice); err != nil {
		return err
	}

	if choice == "" {
		choice = defaultEventType
	}

	for k, v := range eventTypes {
		if v == choice {
			o.event.EventType = k
			return nil
		}
	}

	return fmt.Errorf("invalid event type: %s", choice)
}

func (o *Opts) branchOrTag() error {
	var msg string
	choice := new(string)
	if o.event.BaseBranch != "" {
		return nil
	}

	o.event.BaseBranch = mainBranch

	if o.event.EventType == "pull_request" {
		msg = "Enter the target GIT branch for the Pull Request (default: %s): "
	} else if o.event.EventType == "push" {
		msg = "Enter a target GIT branch or a tag for the push (default: %s)"
	}

	if err := prompt.SurveyAskOne(
		&survey.Input{
			Message: fmt.Sprintf(msg, mainBranch),
		}, choice); err != nil {
		return err
	}

	if *choice != "" {
		o.event.BaseBranch = *choice
	}
	return nil
}

// samplePipeline will try to create a basic pipeline in tekton
// directory.
func (o *Opts) samplePipeline() error {
	cs := o.IOStreams.ColorScheme()

	fname := fmt.Sprintf("%s.yaml", strings.ReplaceAll(o.event.EventType, "_", "-"))
	fpath := filepath.Join(o.GitInfo.TopLevelPath, ".tekton", fname)
	relpath, _ := filepath.Rel(o.GitInfo.TopLevelPath, fpath)

	var reply bool
	msg := fmt.Sprintf("Would you like me to create a basic PipelineRun into the file %s ?", relpath)
	if err := prompt.SurveyAskOne(&survey.Confirm{Message: msg, Default: true}, &reply); err != nil {
		return err
	}

	if !reply {
		return nil
	}

	if _, err := os.Stat(filepath.Join(o.GitInfo.TopLevelPath, ".tekton")); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Join(o.GitInfo.TopLevelPath, ".tekton"), 0o755); err != nil {
			return err
		}
		fmt.Fprintf(o.IOStreams.Out, "%s Directory %s has been created.\n",
			cs.InfoIcon(),
			cs.Bold(".tekton"),
		)
	}

	if _, err := os.Stat(fpath); !os.IsNotExist(err) {
		var overwrite bool
		msg := fmt.Sprintf("There is already a file named: %s would you like me to override it?", fpath)
		if err := prompt.SurveyAskOne(&survey.Confirm{Message: msg, Default: false}, &reply); err != nil {
			return err
		}
		if !overwrite {
			return nil
		}
	}

	tmpl, err := o.genTmpl()
	if err != nil {
		return err
	}

	// nolint: gosec
	err = ioutil.WriteFile(fpath, tmpl.Bytes(), 0o644)
	if err != nil {
		return err
	}

	fmt.Fprintf(o.IOStreams.Out, "%s A basic template has been created in %s, feel free to customize it.\n",
		cs.SuccessIcon(),
		cs.Bold(fpath),
	)
	fmt.Fprintf(o.IOStreams.Out, "%s You can test your pipeline manually with: ", cs.InfoIcon())
	fmt.Fprintf(o.IOStreams.Out, "tkn-pac resolve -f %s | kubectl create -f-\n", relpath)

	return nil
}
