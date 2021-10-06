package repository

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/git"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/ui"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const defaultMainBranch = "main"

type CreateOptions struct {
	RepositoryName          string
	Namespace, TargetURL    string
	EventType, TargetBranch string

	CurrentNS string

	IOStreams *ui.IOStreams
	Run       *params.Run
	CLIOpts   *params.PacCliOpts
	AssumeYes bool
}

func CreateCommand(run *params.Run, ioStreams *ui.IOStreams) *cobra.Command {
	createOpts := CreateOptions{}
	cmd := &cobra.Command{
		Use:     "create",
		Aliases: []string{"new"},
		Short:   "Create  a repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			createOpts.IOStreams = ioStreams
			createOpts.CLIOpts, err = params.NewCliOptions(cmd)
			if err != nil {
				return err
			}
			createOpts.IOStreams.SetColorEnabled(!createOpts.CLIOpts.NoColoring)
			err = run.Clients.NewClients(&run.Info)
			if err != nil {
				return err
			}
			createOpts.Run = run
			createOpts.CurrentNS = run.Info.Kube.Namespace
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			return create(context.Background(), cwd, createOpts)
		},
	}

	cmd.PersistentFlags().BoolP(noColorFlag, "C", !ioStreams.ColorEnabled(), "disable coloring")
	cmd.PersistentFlags().StringVar(&createOpts.RepositoryName, "name", "", "The repository name")
	cmd.PersistentFlags().StringVar(&createOpts.TargetBranch, "branch", "", "The target branch of the repository  event to handle (eg: main, nightly)")
	cmd.PersistentFlags().StringVar(&createOpts.EventType, "event-type", "", "The event type of the repository event to handle (eg: pull_request, push)")
	cmd.PersistentFlags().StringVar(&createOpts.TargetURL, "url", "", "The repository URL from where the event will come from")
	cmd.PersistentFlags().StringVarP(&createOpts.Namespace, "namespace", "n", "", "The target namespace where the runs will be created")
	cmd.PersistentFlags().BoolVarP(&createOpts.AssumeYes, "assume-yes", "y", false,
		"Do not ask questions and just assume yes to everything")

	return cmd
}

// askToCreateSimplePipeline will try to create a basic pipeline in tekton
// directory.
func askToCreateSimplePipeline(gitRoot string, opts CreateOptions) error {
	fpath := filepath.Join(gitRoot, ".tekton", fmt.Sprintf("%s.yaml", opts.EventType))
	cwd, _ := os.Getwd()
	abspath, _ := filepath.Rel(cwd, fpath)

	reply, err := askYesNo(opts,
		fmt.Sprintf("Would you like me to create a basic PipelineRun file into the file %s ?", abspath),
		"True")
	if err != nil {
		return err
	}

	if !reply {
		return nil
	}

	if _, err = os.Stat(filepath.Join(gitRoot, ".tekton")); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Join(gitRoot, ".tekton"), 0o755); err != nil {
			return err
		}
	}

	if _, err = os.Stat(fpath); !os.IsNotExist(err) {
		overwrite, err := askYesNo(opts,
			fmt.Sprintf("There is already a file named: %s would you like me to override it?", fpath), "No")
		if err != nil {
			return err
		}
		if !overwrite {
			return nil
		}
	}

	tmpl := fmt.Sprintf(`---
apiVersion: tekton.dev/v1beta1
kind: PipelineRun
metadata:
  name: %s
  annotations:
    # The event we are targeting (ie: pull_request, push)
    pipelinesascode.tekton.dev/on-event: "[%s]"

    # The branch or tag we are targeting (ie: main, refs/tags/*)
    pipelinesascode.tekton.dev/on-target-branch: "[%s]"

    # Fetch the git-clone task from hub, we are able to reference it with taskRef
    pipelinesascode.tekton.dev/task: "[git-clone]"

    # You can add more tasks in here to reuse, browse the one you like from here
    # https://hub.tekton.dev/
    # example:
    # pipelinesascode.tekton.dev/task-1: "[maven, buildah]"

    # How many runs we want to keep attached to this event
    pipelinesascode.tekton.dev/max-keep-runs: "5"
spec:
  params:
    # The variable with brackets are special to Pipelines as Code
    # They will automatically be expanded with the events from Github.
    - name: repo_url
      value: "{{repo_url}}"
    - name: revision
      value: "{{revision}}"
  pipelineSpec:
    params:
      - name: repo_url
      - name: revision
    workspaces:
      - name: source
      - name: basic-auth
    tasks:
      - name: fetch-repository
        taskRef:
          name: git-clone
        workspaces:
          - name: output
            workspace: source
          - name: basic-auth
            workspace: basic-auth
        params:
          - name: url
            value: $(params.repo_url)
          - name: revision
            value: $(params.revision)
      # Customize this task if you like, or just do a taskRef
      # to one of the hub task.
      - name: noop-task
        runAfter:
          - fetch-repository
        workspaces:
          - name: source
            workspace: source
        taskSpec:
          workspaces:
            - name: source
          steps:
            - name: noop-task
              image: registry.access.redhat.com/ubi8/ubi-micro:8.4
              workingDir: $(workspaces.source.path)
              script: |
                exit 0
  workspaces:
  - name: source
    volumeClaimTemplate:
      spec:
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 1Gi
  # This workspace will inject secret to help the git-clone task to be able to
  # checkout the private repositories
  - name: basic-auth
    secret:
      secretName: "pac-git-basic-auth-{{repo_owner}}-{{repo_name}}"
      `, opts.RepositoryName, opts.EventType, opts.TargetBranch)
	// nolint: gosec
	err = ioutil.WriteFile(fpath, []byte(tmpl), 0o644)
	if err != nil {
		return err
	}

	cs := opts.IOStreams.ColorScheme()
	fmt.Fprintf(opts.IOStreams.Out, "%s A basic template has been created in %s, feel free to customize it.\n",
		cs.SuccessIcon(),
		cs.Bold(fpath),
	)
	fmt.Fprintf(opts.IOStreams.Out, "%s You can test your pipeline manually with :.\n", cs.InfoIcon())
	fmt.Fprintf(opts.IOStreams.Out, "tkn-pac resolve --generateName \\\n"+
		"     --params revision=%s --params repo_url=\"%s\" \\\n      -f %s | k create -f-\n", opts.TargetBranch, opts.TargetURL, fpath)

	return nil
}

func askYesNo(opts CreateOptions, question string, defaults string) (bool, error) {
	var answer string
	if opts.AssumeYes {
		return true, nil
	}
	err := opts.CLIOpts.Ask([]*survey.Question{
		{
			Prompt: &survey.Select{
				Options: []string{"Yes", "No"},
				Default: defaults,
				Message: question,
			},
		},
	}, &answer)
	if err != nil {
		return false, err
	}

	if answer == "True" || answer == "Yes" {
		return true, nil
	}

	return false, nil
}

// create ...
func create(ctx context.Context, gitdir string, opts CreateOptions) error {
	var qs []*survey.Question
	var err error

	gitinfo := git.GetGitInfo(gitdir)

	if opts.AssumeYes && opts.Namespace == "" {
		opts.Namespace = opts.CurrentNS
	}
	if opts.AssumeYes && opts.TargetURL == "" {
		opts.TargetURL = gitinfo.TargetURL
	}
	if opts.AssumeYes && opts.TargetBranch == "" {
		opts.TargetBranch = defaultMainBranch
	}
	if opts.AssumeYes && opts.EventType == "" {
		opts.EventType = "pull_request"
	}

	if opts.Namespace == "" {
		qs = append(qs, &survey.Question{
			Name:   "Namespace",
			Prompt: &survey.Input{Message: fmt.Sprintf("Enter the namespace where the pipeline should run (default: %s): ", opts.CurrentNS)},
		})
	}
	if opts.TargetURL == "" {
		prompt := "Enter the target url: "
		if gitinfo.TargetURL != "" {
			prompt = fmt.Sprintf("Enter the Git repository url containing the pipelines (default: %s): ", gitinfo.TargetURL)
		}
		qs = append(qs, &survey.Question{
			Name:   "TargetURL",
			Prompt: &survey.Input{Message: prompt},
		})
	}

	if opts.TargetBranch == "" {
		qs = append(qs, &survey.Question{
			Name:   "TargetBranch",
			Prompt: &survey.Input{Message: "Enter the target GIT branch (default: main): "},
		})
	}

	if opts.EventType == "" {
		qs = append(qs, &survey.Question{
			Name: "EventType",
			Prompt: &survey.Select{
				Message: "Enter the Git event type for triggering the pipeline: ",
				Options: []string{"pull_request", "push"},
				Default: "pull_request",
			},
		})
	}

	if qs != nil {
		err := opts.CLIOpts.Ask(qs, &opts)
		if err != nil {
			return err
		}
	}
	if opts.Namespace == "" {
		opts.Namespace = opts.CurrentNS
	}
	if opts.TargetURL == "" && gitinfo.TargetURL != "" {
		opts.TargetURL = gitinfo.TargetURL
	} else if opts.TargetURL == "" {
		return fmt.Errorf("we didn't get a target URL")
	}
	if opts.TargetBranch == "" {
		opts.TargetBranch = defaultMainBranch
	}
	if opts.RepositoryName == "" {
		opts.RepositoryName, err = askNameForResource(opts, "Enter the repository name")
		if err != nil {
			return err
		}
	}
	cs := opts.IOStreams.ColorScheme()
	if opts.Namespace != opts.CurrentNS {
		if err := askCreateNamespace(ctx, opts, cs); err != nil {
			return err
		}
	}
	_, err = opts.Run.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(opts.Namespace).Create(ctx,
		&v1alpha1.Repository{
			ObjectMeta: metav1.ObjectMeta{
				Name: opts.RepositoryName,
			},
			Spec: v1alpha1.RepositorySpec{
				URL:       opts.TargetURL,
				EventType: opts.EventType,
				Branch:    opts.TargetBranch,
			},
		},
		metav1.CreateOptions{})
	if err != nil {
		return err
	}
	fmt.Fprintf(opts.IOStreams.Out, "%s Repository %s has been created in %s namespace\n",
		cs.SuccessIconWithColor(cs.Green),
		opts.RepositoryName,
		opts.Namespace,
	)

	if err := askToCreateSimplePipeline(gitinfo.TopLevelPath, opts); err != nil {
		return err
	}

	fmt.Fprintf(opts.IOStreams.Out, "%s Don't forget to install the GitHub application into your repo %s\n",
		cs.InfoIcon(),
		opts.TargetURL,
	)
	fmt.Fprintf(opts.IOStreams.Out, "%s and we are done! enjoy :)))\n", cs.SuccessIcon())

	return nil
}

func askNameForResource(opts CreateOptions, question string) (string, error) {
	s, err := ui.GetRepoOwnerFromGHURL(opts.TargetURL)
	repo := fmt.Sprintf("%s-%s", filepath.Base(s), strings.ReplaceAll(opts.EventType, "_", "-"))
	// Don't ask question if we auto generated
	if opts.AssumeYes {
		return repo, nil
	}

	if err == nil {
		// No assume yes but generated a name propery so let's return that
		return repo, nil
	}

	repo = ""
	err = opts.CLIOpts.Ask([]*survey.Question{
		{
			Prompt: &survey.Input{Message: question},
		},
	}, &repo)
	if err != nil {
		return "", err
	}
	if repo == "" {
		return "", fmt.Errorf("no name has been set")
	}
	return repo, nil
}

func askCreateNamespace(ctx context.Context, opts CreateOptions, cs *ui.ColorScheme) error {
	_, err := opts.Run.Clients.Kube.CoreV1().Namespaces().Get(ctx, opts.Namespace, metav1.GetOptions{})
	if err != nil {
		fmt.Fprintf(opts.IOStreams.Out, "%s Namespace %s is not created yet\n",
			cs.WarningIcon(),
			opts.Namespace,
		)
		createNamespace, err := askYesNo(opts,
			fmt.Sprintf("Would you like me to create the namespace %s?", opts.Namespace),
			"Yes")
		if err != nil {
			return err
		}
		if !createNamespace {
			return fmt.Errorf("you need to create the target namespace")
		}
		_, err = opts.Run.Clients.Kube.CoreV1().Namespaces().Create(ctx,
			&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: opts.Namespace}},
			metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}
