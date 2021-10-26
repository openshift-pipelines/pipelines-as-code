package generate

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/git"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/spf13/cobra"
)

var (
	eventTypes       = map[string]string{"pull_request": "Pull Request", "push": "Push to a Branch or a Tag"}
	defaultEventType = "Pull Request"
	mainBranch       = "main"
)

type generateOpts struct {
	event      *info.Event
	repository *apipac.Repository
	run        *params.Run
	gitInfo    *git.Info

	ioStreams *cli.IOStreams
	cliOpts   *cli.PacCliOpts
}

func Command(run *params.Run, ioStreams *cli.IOStreams) *cobra.Command {
	opt := &generateOpts{
		event:      &info.Event{},
		repository: &apipac.Repository{},
		ioStreams:  ioStreams,
		run:        run,
	}
	cmd := &cobra.Command{
		Use:     "generate",
		Aliases: []string{"gen"},
		Short:   "Generate PipelineRun",
		RunE: func(cmd *cobra.Command, args []string) error {
			opt.cliOpts = cli.NewCliOptions(cmd)
			opt.ioStreams.SetColorEnabled(!opt.cliOpts.NoColoring)

			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			opt.gitInfo = git.GetGitInfo(cwd)
			if err := run.Clients.NewClients(&run.Info); err != nil {
				return err
			}

			if err := opt.getTargetEvent(); err != nil {
				return err
			}

			if err := opt.getBranchOrTag(); err != nil {
				return err
			}

			if err := opt.generateSamplePipeline(); err != nil {
				return err
			}

			return nil
		},
	}
	return cmd
}

func (o *generateOpts) getTargetEvent() error {
	msg := "Enter the Git event type for triggering the pipeline: "

	eventLabels := make([]string, 0, len(eventTypes))
	for _, label := range eventTypes {
		eventLabels = append(eventLabels, label)
	}

	choice := new(string)
	err := o.cliOpts.Ask([]*survey.Question{
		{
			Prompt: &survey.Select{
				Message: msg,
				Options: eventLabels,
				Default: defaultEventType,
			},
		},
	}, choice)
	if err != nil {
		return err
	}

	for k, v := range eventTypes {
		if v == *choice {
			o.event.EventType = k
		}
	}

	return nil
}

func (o *generateOpts) getBranchOrTag() error {
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

	err := o.cliOpts.Ask([]*survey.Question{
		{
			Prompt: &survey.Input{
				Message: fmt.Sprintf(msg, mainBranch),
			},
		},
	}, choice)
	if err != nil {
		return err
	}

	if *choice != "" {
		o.event.BaseBranch = *choice
	}
	return nil
}

// generateSamplePipeline will try to create a basic pipeline in tekton
// directory.
func (o *generateOpts) generateSamplePipeline() error {
	cs := o.ioStreams.ColorScheme()

	fname := fmt.Sprintf("%s.yaml", strings.ReplaceAll(o.event.EventType, "_", "-"))
	fpath := filepath.Join(o.gitInfo.TopLevelPath, ".tekton", fname)
	relpath, _ := filepath.Rel(o.gitInfo.TopLevelPath, fpath)

	reply, err := cli.AskYesNo(
		o.cliOpts,
		fmt.Sprintf("Would you like me to create a basic PipelineRun into the file %s ?", relpath),
		true)
	if err != nil {
		return err
	}

	if !reply {
		return nil
	}

	if _, err = os.Stat(filepath.Join(o.gitInfo.TopLevelPath, ".tekton")); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Join(o.gitInfo.TopLevelPath, ".tekton"), 0o755); err != nil {
			return err
		}
		fmt.Fprintf(o.ioStreams.Out, "%s Directory %s has been created.\n",
			cs.InfoIcon(),
			cs.Bold(".tekton"),
		)
	}

	if _, err = os.Stat(fpath); !os.IsNotExist(err) {
		overwrite, err := cli.AskYesNo(o.cliOpts,
			fmt.Sprintf("There is already a file named: %s would you like me to override it?", fpath),
			false)
		if err != nil {
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

	err = ioutil.WriteFile(fpath, tmpl.Bytes(), 0o644)
	if err != nil {
		return err
	}

	fmt.Fprintf(o.ioStreams.Out, "%s A basic template has been created in %s, feel free to customize it.\n",
		cs.SuccessIcon(),
		cs.Bold(fpath),
	)
	fmt.Fprintf(o.ioStreams.Out, "%s You can test your pipeline manually with: ", cs.InfoIcon())
	fmt.Fprintf(o.ioStreams.Out, "tkn-pac resolve -f %s | kubectl create -f-\n", relpath)

	return nil
}

// // create ...
// func create(ctx context.Context, gitdir string, opts CreateOptions) error {
// 	var qs []*survey.Question
// 	var err error

// 	gitinfo := git.GetGitInfo(gitdir)

// 	if opts.AssumeYes && opts.repository.GetNamespace() == "" {
// 		opts.repository.Namespace = opts.CurrentNS
// 	}
// 	if opts.AssumeYes && opts.event.URL == "" {
// 		opts.event.URL = gitinfo.TargetURL
// 	}
// 	if opts.AssumeYes && opts.event.BaseBranch == "" {
// 		opts.event.BaseBranch = defaultMainBranch
// 	}
// 	if opts.AssumeYes && opts.event.EventType == "" {
// 		opts.event.EventType = "pull_request"
// 	}

// 	if opts.repository.GetNamespace() == "" {
// 		qs = append(qs, &survey.Question{
// 			Name:   "Namespace",
// 			Prompt: &survey.Input{Message: fmt.Sprintf("Enter the namespace where the pipeline should run (default: %s): ", opts.CurrentNS)},
// 		})
// 	}
// 	if opts.event.URL == "" {
// 		prompt := "Enter the target url: "
// 		if gitinfo.TargetURL != "" {
// 			prompt = fmt.Sprintf("Enter the Git repository url containing the pipelines (default: %s): ", gitinfo.TargetURL)
// 		}
// 		qs = append(qs, &survey.Question{
// 			Name:   "TargetURL",
// 			Prompt: &survey.Input{Message: prompt},
// 		})
// 	}

// 	if qs != nil {
// 		err := opts.CLIOpts.Ask(qs, &opts)
// 		if err != nil {
// 			return err
// 		}
// 	}

// 	if opts.repository.GetNamespace() == "" {
// 		opts.repository.Namespace = opts.CurrentNS
// 	}

// 	if opts.repository.GetNamespace() == "" {
// 		opts.repository.Namespace, err = askNameForResource(opts, "Enter the repository name")
// 		if err != nil {
// 			return err
// 		}
// 	}

// 	if opts.event.URL == "" && gitinfo.TargetURL != "" {
// 		opts.event.URL = gitinfo.TargetURL
// 	} else if opts.event.URL == "" {
// 		return fmt.Errorf("we didn't get a target URL")
// 	}

// 	cs := opts.IOStreams.ColorScheme()
// 	if opts.repository.GetNamespace() != opts.CurrentNS {
// 		if err := askCreateNamespace(ctx, opts, cs); err != nil {
// 			return err
// 		}
// 	}
// 	opts.repository.Spec = apipac.RepositorySpec{
// 		URL: opts.event.URL,
// 	}

// 	_, err = opts.Run.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(opts.repository.GetNamespace()).Create(ctx,
// 		opts.repository,
// 		metav1.CreateOptions{})
// 	if err != nil {
// 		return err
// 	}
// 	fmt.Fprintf(opts.IOStreams.Out, "%s Repository %s has been created in %s namespace\n",
// 		cs.SuccessIconWithColor(cs.Green),
// 		opts.repository.GetName(),
// 		opts.repository.GetNamespace(),
// 	)

// 	if err := askToCreateSimplePipeline(gitinfo.TopLevelPath, opts); err != nil {
// 		return err
// 	}

// 	fmt.Fprintf(opts.IOStreams.Out, "%s Don't forget to install the GitHub application into your repo %s\n",
// 		cs.InfoIcon(),
// 		opts.event.URL,
// 	)
// 	fmt.Fprintf(opts.IOStreams.Out, "%s and we are done! enjoy :)))\n", cs.SuccessIcon())

// 	return nil
// }

// func askNameForResource(opts CreateOptions, question string) (string, error) {
// 	s, err := ui.GetRepoOwnerFromGHURL(opts.event.URL)
// 	repo := fmt.Sprintf("%s-%s", filepath.Base(s), strings.ReplaceAll(opts.event.EventType, "_", "-"))
// 	// Don't ask question if we auto generated
// 	if opts.AssumeYes {
// 		return repo, nil
// 	}

// 	if err == nil {
// 		// No assume yes but generated a name properly so let's return that
// 		return repo, nil
// 	}

// 	repo = ""
// 	err = opts.CLIOpts.Ask([]*survey.Question{
// 		{
// 			Prompt: &survey.Input{Message: question},
// 		},
// 	}, &repo)
// 	if err != nil {
// 		return "", err
// 	}
// 	if repo == "" {
// 		return "", fmt.Errorf("no name has been set")
// 	}
// 	return repo, nil
// }
