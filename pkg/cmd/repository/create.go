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
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli/ui"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/flags"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/git"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const defaultMainBranch = "main"

type CreateOptions struct {
	RepositoryName             string
	TargetNamespace, TargetURL string
	EventType, TargetBranch    string

	CurrentNS string

	IOStreams *ui.IOStreams
	Clients   *cli.Clients
	CLIOpts   *flags.CliOpts
	AssumeYes bool
}

func CreateCommand(p cli.Params, ioStreams *ui.IOStreams) *cobra.Command {
	createOpts := CreateOptions{}
	cmd := &cobra.Command{
		Use:     "create",
		Aliases: []string{"new"},
		Short:   "Create  a repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			createOpts.IOStreams = ioStreams
			createOpts.CLIOpts, err = flags.NewCliOptions(cmd)
			if err != nil {
				return err
			}
			createOpts.IOStreams.SetColorEnabled(!createOpts.CLIOpts.NoColoring)
			createOpts.Clients, err = p.Clients()
			if err != nil {
				return err
			}
			createOpts.CurrentNS = p.GetNamespace()
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			return create(context.Background(), cwd, createOpts)
		},
	}
	cmd.PersistentFlags().StringVar(&createOpts.RepositoryName, "name", "", "The repository name")
	cmd.PersistentFlags().StringVar(&createOpts.TargetBranch, "branch", "", "The target branch of the repository  event to handle (eg: main, nightly)")
	cmd.PersistentFlags().StringVar(&createOpts.EventType, "event-type", "", "The event type of the repository event to handle (eg: pull_request, push)")
	cmd.PersistentFlags().StringVar(&createOpts.TargetURL, "url", "", "The repository URL from where the event will come from")
	cmd.PersistentFlags().StringVar(&createOpts.TargetNamespace, "target-namespace", "", "The target namespace where the runs will be created")
	cmd.PersistentFlags().BoolVarP(&createOpts.AssumeYes, "assume-yes", "y", false,
		"Do not ask questions and just assume yes to everything")

	return cmd
}

// askToCreateSimplePipeline will try to create a basic pipeline in tekton
// directory.
func askToCreateSimplePipeline(gitRoot string, opts CreateOptions) error {
	fpath := filepath.Join(gitRoot, ".tekton", fmt.Sprintf("%s.yaml", opts.EventType))

	reply, err := askYesNo(opts,
		fmt.Sprintf("Would you like to create a basic PipelineRun file: %s in your createBasicPipelineRun?", fpath),
		"True")
	if err != nil {
		return err
	}
	if !reply {
		return nil
	}

	if err := os.MkdirAll(filepath.Join(gitRoot, ".tekton"), 0o755); err != nil {
		return err
	}
	if _, err = os.Stat(fpath); err != nil && !os.IsNotExist(err) {
		overwrite, err := askYesNo(opts,
			fmt.Sprintf("There is already a file named: %s would you like to override it?", fpath), "No")
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

    # The branch we are targeting (ie: main)
    pipelinesascode.tekton.dev/on-target-branch: "[%s]"

    # Fetch the git-clone task from hub, we are able to reference it with taskRef
    pipelinesascode.tekton.dev/task: "[git-clone]"

    # How many runs we want to keep attached to this event
    pipelinesascode.tekton.dev/max-keep-runs: "5"
spec:
  params:
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
    tasks:
      - name: fetch-repository
        taskRef:
          name: git-clone
        workspaces:
          - name: output
            workspace: source
        params:
          - name: url
            value: $(params.repo_url)
          - name: revision
            value: $(params.revision)
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

	if strings.ToLower(answer) == "yes" {
		return true, nil
	}

	return false, nil
}

// create ...
func create(ctx context.Context, gitdir string, opts CreateOptions) error {
	var qs []*survey.Question
	var err error

	gitinfo := git.GetGitInfo(gitdir)

	if opts.AssumeYes && opts.TargetNamespace == "" {
		opts.TargetNamespace = opts.CurrentNS
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

	if opts.TargetNamespace == "" {
		qs = append(qs, &survey.Question{
			Name:   "TargetNamespace",
			Prompt: &survey.Input{Message: fmt.Sprintf("Enter the target namespace (default: %s):", opts.CurrentNS)},
		})
	}
	if opts.TargetURL == "" {
		prompt := "Enter the target url: "
		if gitinfo.TargetURL != "" {
			prompt = fmt.Sprintf("Enter the target url (default: %s): ", gitinfo.TargetURL)
		}
		qs = append(qs, &survey.Question{
			Name:   "TargetURL",
			Prompt: &survey.Input{Message: prompt},
		})
	}

	if opts.TargetBranch == "" {
		qs = append(qs, &survey.Question{
			Name:   "TargetBranch",
			Prompt: &survey.Input{Message: "Enter the target branch (default: main): "},
		})
	}

	if opts.EventType == "" {
		qs = append(qs, &survey.Question{
			Name: "EventType",
			Prompt: &survey.Select{
				Message: "What type of webhook event:",
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
	if opts.TargetNamespace == "" {
		opts.TargetNamespace = opts.CurrentNS
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
		opts.RepositoryName, err = askNameForResource(opts)
		if err != nil {
			return err
		}
	}
	cs := opts.IOStreams.ColorScheme()
	if opts.TargetNamespace != opts.CurrentNS {
		if err := askCreateNamespace(ctx, opts, cs); err != nil {
			return err
		}
	}
	_, err = opts.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(opts.TargetNamespace).Create(ctx,
		&v1alpha1.Repository{
			ObjectMeta: metav1.ObjectMeta{
				Name: opts.RepositoryName,
			},
			Spec: v1alpha1.RepositorySpec{
				Namespace: opts.TargetNamespace,
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
		opts.TargetNamespace,
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

func askNameForResource(opts CreateOptions) (string, error) {
	s, err := ui.GetRepoOwnerFromGHURL(opts.TargetURL)
	generatedNS := fmt.Sprintf("%s-%s", filepath.Base(s), strings.ReplaceAll(opts.EventType, "_", "-"))
	if opts.AssumeYes {
		return generatedNS, nil
	}
	prompt := fmt.Sprintf("Set a name for this resource (default: %s):", generatedNS)
	if err != nil {
		prompt = "Set a name for this resource:"
		generatedNS = ""
	}

	var repo string
	err = opts.CLIOpts.Ask([]*survey.Question{
		{
			Prompt: &survey.Input{Message: prompt},
		},
	}, &repo)
	if err != nil {
		return "", err
	}
	if repo == "" && generatedNS == "" {
		return "", fmt.Errorf("no name has been set")
	}
	if repo == "" {
		repo = generatedNS
	}
	return repo, nil
}

func askCreateNamespace(ctx context.Context, opts CreateOptions, cs *ui.ColorScheme) error {
	_, err := opts.Clients.Kube.CoreV1().Namespaces().Get(ctx, opts.TargetNamespace, metav1.GetOptions{})
	if err != nil {
		fmt.Fprintf(opts.IOStreams.Out, "%s Namespace %s is not created yet\n",
			cs.WarningIcon(),
			opts.TargetNamespace,
		)
		createNamespace, err := askYesNo(opts,
			fmt.Sprintf("Would you like me to create the namespace %s?", opts.TargetNamespace),
			"Yes")
		if err != nil {
			return err
		}
		if !createNamespace {
			return fmt.Errorf("you need to create the target namespace")
		}
		_, err = opts.Clients.Kube.CoreV1().Namespaces().Create(ctx,
			&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: opts.TargetNamespace}},
			metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}
