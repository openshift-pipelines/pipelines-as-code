package generate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli/prompt"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/git"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var eventTypes = map[string]string{triggertype.PullRequest.String(): "Pull Request", "push": "Push to a Branch or a Tag"}

const (
	gitCloneClusterTaskName = "git-clone"
	defaultEventType        = "Pull Request"
	mainBranch              = "main"
)

type Opts struct {
	Event   *info.Event
	GitInfo *git.Info

	IOStreams *cli.IOStreams
	CLIOpts   *cli.PacCliOpts

	pipelineRunName         string
	FileName                string
	overwrite               bool
	language                string
	generateWithClusterTask bool
}

func MakeOpts() *Opts {
	return &Opts{
		Event:   info.NewEvent(),
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
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx := context.Background()
			gopt.CLIOpts = cli.NewCliOptions()
			gopt.IOStreams.SetColorEnabled(!gopt.CLIOpts.NoColoring)

			if gopt.generateWithClusterTask {
				if err := run.Clients.NewClients(ctx, &run.Info); err != nil {
					// if we don't have access to the cluster we can't do much about it
					gopt.generateWithClusterTask = false
				} else {
					// NOTE(chmou): This is for v1beta1, we need to figure out how to do this for v1.
					// Trying to find resolver with that same name?
					_, err := run.Clients.Tekton.TektonV1beta1().ClusterTasks().Get(ctx,
						gitCloneClusterTaskName,
						metav1.GetOptions{})
					if err == nil {
						gopt.generateWithClusterTask = true
					} else {
						gopt.generateWithClusterTask = false
					}
				}
			}
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			gopt.GitInfo = git.GetGitInfo(cwd)
			return Generate(gopt, true)
		},
		Annotations: map[string]string{
			"commandType": "main",
		},
	}
	cmd.PersistentFlags().StringVar(&gopt.Event.BaseBranch, "branch", "",
		"The target branch for the PipelineRun to handle (eg: main, nightly)")
	cmd.PersistentFlags().StringVar(&gopt.Event.EventType, "event-type", "",
		"The event type of the repository event to handle (eg: pull_request, push)")
	cmd.PersistentFlags().StringVar(&gopt.pipelineRunName, "pipeline-name", "",
		"The pipeline name")
	cmd.PersistentFlags().StringVarP(&gopt.FileName, "file-name", "f", "",
		"The file name location")
	cmd.PersistentFlags().BoolVar(&gopt.overwrite, "overwrite", false,
		"Whether to overwrite the file if it exist")
	cmd.PersistentFlags().StringVarP(&gopt.language, "language", "l", "",
		"Generate for this programming language")
	cmd.PersistentFlags().BoolVarP(&gopt.generateWithClusterTask, "use-clustertasks", "", true,
		"By default we try to use the clustertasks unless not available")
	return cmd
}

func Generate(o *Opts, recreateTemplate bool) error {
	if err := o.targetEvent(); err != nil {
		return err
	}

	if err := o.branchOrTag(); err != nil {
		return err
	}

	return o.samplePipeline(recreateTemplate)
}

func (o *Opts) targetEvent() error {
	var choice string
	if o.Event.EventType != "" {
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
			o.Event.EventType = k
			return nil
		}
	}

	return fmt.Errorf("invalid event type: %s", choice)
}

func (o *Opts) branchOrTag() error {
	var msg string
	choice := new(string)
	if o.Event.BaseBranch != "" {
		return nil
	}

	o.Event.BaseBranch = mainBranch

	if o.Event.EventType == triggertype.PullRequest.String() {
		msg = "Enter the target GIT branch for the Pull Request (default: %s): "
	} else if o.Event.EventType == "push" {
		msg = "Enter a target GIT branch or a tag for the push (default: %s)"
	}

	if err := prompt.SurveyAskOne(
		&survey.Input{
			Message: fmt.Sprintf(msg, mainBranch),
		}, choice); err != nil {
		return err
	}

	if *choice != "" {
		o.Event.BaseBranch = *choice
	}
	return nil
}

func generatefileName(eventType string) string {
	var filename string
	types := strings.Split(eventType, ",")
	if len(types) > 1 {
		filename = "pipelinerun"
	} else {
		filename = strings.ReplaceAll(eventType, "_", "-")
	}
	return fmt.Sprintf("%s.yaml", filename)
}

// samplePipeline will try to create a basic pipeline in tekton
// directory.
func (o *Opts) samplePipeline(recreateTemplate bool) error {
	cs := o.IOStreams.ColorScheme()
	var relpath, fpath string

	if o.FileName != "" {
		fpath = o.FileName
		relpath = fpath
	} else {
		fname := generatefileName(o.Event.EventType)
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
		if recreateTemplate {
			var overwrite bool
			msg := fmt.Sprintf("There is already a file named: %s would you like me to override it?", relpath)
			if err := prompt.SurveyAskOne(&survey.Confirm{Message: msg, Default: false}, &overwrite); err != nil {
				return err
			}
			if !overwrite {
				fmt.Fprintf(o.IOStreams.ErrOut, "%s Not overwriting file, exiting...\n", cs.WarningIcon())
				fmt.Fprintf(o.IOStreams.ErrOut, "%s Feel free to use the -f flag if you want to target another file name\n...", cs.InfoIcon())
			}
		} else {
			fmt.Fprintf(o.IOStreams.Out, "%s There is already a file named: %s, skipping template generation, feel free to use \"%s pac generate\" command to generate sample template.\n", cs.InfoIcon(), relpath,
				settings.TknBinaryName)
		}
		return nil
	}
	tmpl, err := o.genTmpl()
	if err != nil {
		return err
	}

	//nolint: gosec
	err = os.WriteFile(fpath, tmpl.Bytes(), 0o644)
	if err != nil {
		return fmt.Errorf("cannot write template to %s: %w", fpath, err)
	}

	fmt.Fprintf(o.IOStreams.Out, "%s A basic template has been created in %s, feel free to customize it.\n",
		cs.SuccessIcon(),
		cs.Bold(fpath),
	)
	fmt.Fprintf(o.IOStreams.Out, "%s You can test your pipeline by pushing the generated template to your git repository\n", cs.InfoIcon())

	return nil
}
