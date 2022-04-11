package generate

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli/prompt"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/git"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var eventTypes = map[string]string{"pull_request": "Pull Request", "push": "Push to a Branch or a Tag"}

const (
	gitCloneClusterTaskName = "git-clone"
	defaultEventType        = "Pull Request"
	mainBranch              = "main"
)

type Opts struct {
	event   *info.Event
	GitInfo *git.Info

	IOStreams *cli.IOStreams
	CLIOpts   *cli.PacCliOpts

	pipelineRunName         string
	fileName                string
	overwrite               bool
	language                string
	generateWithClusterTask bool
}

func MakeOpts() *Opts {
	return &Opts{
		event:   &info.Event{},
		GitInfo: &git.Info{},

		IOStreams: &cli.IOStreams{},
		CLIOpts:   &cli.PacCliOpts{},
	}
}

func Command(run *params.Run, ioStreams *cli.IOStreams) *cobra.Command {
	gopt := MakeOpts()
	gopt.IOStreams = ioStreams
	cmd := &cobra.Command{
		Use:     "generate",
		Aliases: []string{"gen"},
		Short:   "Generate PipelineRun",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			gopt.CLIOpts = cli.NewCliOptions(cmd)
			gopt.IOStreams.SetColorEnabled(!gopt.CLIOpts.NoColoring)

			if !gopt.generateWithClusterTask {
				if err := run.Clients.NewClients(ctx, &run.Info); err != nil {
					// if we don't have access to the cluster we can't do much about it
					gopt.generateWithClusterTask = false
				} else {
					_, err := run.Clients.Tekton.TektonV1beta1().ClusterTasks().Get(ctx, gitCloneClusterTaskName,
						metav1.GetOptions{})
					if err == nil {
						gopt.generateWithClusterTask = true
					}
				}
			}
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			gopt.GitInfo = git.GetGitInfo(cwd)
			return Generate(gopt)
		},
		Annotations: map[string]string{
			"commandType": "main",
		},
	}
	cmd.PersistentFlags().StringVar(&gopt.event.BaseBranch, "branch", "",
		"The target branch of the repository  event to handle (eg: main, nightly)")
	cmd.PersistentFlags().StringVar(&gopt.event.EventType, "event-type", "",
		"The event type of the repository event to handle (eg: pull_request, push)")
	cmd.PersistentFlags().StringVar(&gopt.event.URL, "url", "",
		"The repository URL from where the event will come from")
	cmd.PersistentFlags().StringVar(&gopt.pipelineRunName, "pipeline-name", "",
		"The pipeline name")
	cmd.PersistentFlags().StringVar(&gopt.fileName, "file-name", "",
		"The file name location")
	cmd.PersistentFlags().BoolVar(&gopt.overwrite, "overwrite", false,
		"Wether to overwrite the file if it exist")
	cmd.PersistentFlags().StringVarP(&gopt.language, "language", "l", "",
		"Generate for this programming language")
	cmd.PersistentFlags().BoolVarP(&gopt.generateWithClusterTask, "use-clustertasks", "", false,
		"By default we will generate the pipeline using task from hub. If you want to use cluster tasks, set this flag")
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
	if o.event.EventType != "" {
		return nil
	}
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
	var relpath, fpath string

	if o.fileName != "" {
		fpath = o.fileName
		relpath = fpath
	} else {
		fname := fmt.Sprintf("%s.yaml", strings.ReplaceAll(o.event.EventType, "_", "-"))
		fpath = filepath.Join(o.GitInfo.TopLevelPath, ".tekton", fname)
		relpath, _ = filepath.Rel(o.GitInfo.TopLevelPath, fpath)
		if _, err := os.Stat(filepath.Join(o.GitInfo.TopLevelPath, ".tekton")); os.IsNotExist(err) {
			if err := os.MkdirAll(filepath.Join(o.GitInfo.TopLevelPath, ".tekton"), 0o755); err != nil {
				return err
			}
			fmt.Fprintf(o.IOStreams.Out, "%s Directory %s has been created.\n",
				cs.InfoIcon(),
				cs.Bold(".tekton"),
			)
		}
	}

	if _, err := os.Stat(fpath); !os.IsNotExist(err) && !o.overwrite {
		var overwrite bool
		msg := fmt.Sprintf("There is already a file named: %s would you like me to override it?", relpath)
		if err := prompt.SurveyAskOne(&survey.Confirm{Message: msg, Default: false}, &overwrite); err != nil {
			return err
		}
		if !overwrite {
			fmt.Fprintf(o.IOStreams.ErrOut, "%s Not overwriting file, exiting...\n", cs.WarningIcon())
			fmt.Fprintf(o.IOStreams.ErrOut, "%s Feel free to use the -f flag if you want to target another file name\n...", cs.InfoIcon())
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
		return fmt.Errorf("cannot write template to %s: %w", fpath, err)
	}

	fmt.Fprintf(o.IOStreams.Out, "%s A basic template has been created in %s, feel free to customize it.\n",
		cs.SuccessIcon(),
		cs.Bold(fpath),
	)
	fmt.Fprintf(o.IOStreams.Out, "%s You can test your pipeline manually with: ", cs.InfoIcon())
	fmt.Fprintf(o.IOStreams.Out, "tkn-pac resolve -f %s | kubectl create -f-\n", relpath)

	return nil
}
