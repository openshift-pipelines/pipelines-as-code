package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/git"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/ui"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type createOptions struct {
	event      *info.Event
	repository *apipac.Repository
	run        *params.Run
	gitInfo    *git.Info

	ioStreams *ui.IOStreams
	cliOpts   *params.PacCliOpts
}

func CreateCommand(run *params.Run, ioStreams *ui.IOStreams) *cobra.Command {
	createOpts := &createOptions{
		event:      &info.Event{},
		repository: &apipac.Repository{},
		run:        run,
	}
	cmd := &cobra.Command{
		Use:     "create",
		Aliases: []string{"new"},
		Short:   "Create  a repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			createOpts.ioStreams = ioStreams
			createOpts.cliOpts, err = params.NewCliOptions(cmd)
			if err != nil {
				return err
			}
			createOpts.ioStreams.SetColorEnabled(!createOpts.cliOpts.NoColoring)

			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			createOpts.gitInfo = git.GetGitInfo(cwd)
			if err := run.Clients.NewClients(&run.Info); err != nil {
				return err
			}

			if err := getRepoURL(createOpts); err != nil {
				return err
			}

			return getOrCreateNamespace(context.Background(), createOpts)
		},
	}

	cmd.PersistentFlags().BoolP(noColorFlag, "C", !ioStreams.ColorEnabled(), "disable coloring")
	cmd.PersistentFlags().StringVar(&createOpts.repository.Name, "name", "", "The repository name")
	cmd.PersistentFlags().StringVar(&createOpts.event.BaseBranch, "branch", "",
		"The target branch of the repository  event to handle (eg: main, nightly)")
	cmd.PersistentFlags().StringVar(&createOpts.event.EventType, "event-type", "",
		"The event type of the repository event to handle (eg: pull_request, push)")
	cmd.PersistentFlags().StringVar(&createOpts.event.URL, "url", "",
		"The repository URL from where the event will come from")
	cmd.PersistentFlags().StringVarP(&createOpts.repository.Namespace, "namespace", "n", "",
		"The target namespace where the runs will be created")

	return cmd
}

// getOrCreateNamespace ask and create namespace or use the default one
func getOrCreateNamespace(ctx context.Context, opts *createOptions) error {
	if opts.repository.Namespace != "" {
		return nil
	}

	// by default, use the current namespace unless it's default or
	// pipelines-as-code and then propose some meaningful namespace based on the
	// git url.
	defaultNamespace := opts.run.Info.Kube.Namespace
	if (defaultNamespace == "default" || defaultNamespace == "pipelines-as-code") &&
		opts.gitInfo.URL != "" {
		defaultNamespace = filepath.Base(opts.gitInfo.URL) + "-pipelines"
	}

	cs := opts.ioStreams.ColorScheme()

	msg := fmt.Sprintf("Please enter the namespace where the pipeline will be created (default: %s):", defaultNamespace)
	err := opts.cliOpts.Ask([]*survey.Question{
		{
			Prompt: &survey.Input{
				Message: msg,
			},
		},
	}, &opts.repository.Namespace)
	if err != nil {
		return err
	}

	// set the namespace as the default one
	if opts.repository.GetNamespace() == "" {
		opts.repository.Namespace = opts.run.Info.Kube.Namespace
	}

	// check if the namespace exists if it does just exit
	if _, err = opts.run.Clients.Kube.CoreV1().Namespaces().Get(
		ctx, defaultNamespace, metav1.GetOptions{}); err == nil {
		return nil
	}

	fmt.Fprintf(opts.ioStreams.Out, "%s Namespace %s is not found\n",
		cs.WarningIcon(),
		defaultNamespace,
	)
	msg = fmt.Sprintf("Would you like me to create the namespace %s?", defaultNamespace)
	createNamespace, err := ui.AskYesNo(opts.cliOpts, msg, true)
	if err != nil {
		return err
	}
	if !createNamespace {
		return fmt.Errorf("you need to create the target namespace")
	}

	_, err = opts.run.Clients.Kube.CoreV1().Namespaces().Create(ctx,
		&v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: defaultNamespace,
			},
		},
		metav1.CreateOptions{})
	return err
}

// getRepoURL get the repository URL from the user using the git url as default.
func getRepoURL(opts *createOptions) error {
	if opts.event.URL != "" {
		return nil
	}

	prompt := "Enter the Git repository url containing the pipelines "
	if opts.gitInfo.URL != "" {
		prompt += fmt.Sprintf("(default: %s)", opts.gitInfo.URL)
	}
	prompt += ": "
	if err := opts.cliOpts.Ask([]*survey.Question{
		{
			Prompt: &survey.Input{
				Message: prompt,
			},
		},
	}, &opts.event.URL); err != nil {
		return err
	}
	if opts.event.URL == "" && opts.gitInfo.URL != "" {
		opts.event.URL = opts.gitInfo.URL
		return nil
	}
	return fmt.Errorf("no url has been provided")
}
