package repository

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli/ui"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/flags"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CreateOptions struct {
	RepositoryName             string
	TargetNamespace, TargetURL string
	EventType, TargetBranch    string

	CurrentNS string

	IOStreams *ui.IOStreams
	Clients   *cli.Clients
	CLIOpts   *flags.CliOpts
}

func CreateCommand(p cli.Params) *cobra.Command {
	createOpts := CreateOptions{}
	cmd := &cobra.Command{
		Use:     "create",
		Aliases: []string{"new"},
		Short:   "Create  a repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error

			createOpts.IOStreams = ui.NewIOStreams()
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
			return create(context.Background(), createOpts)
		},
	}
	cmd.PersistentFlags().StringVar(&createOpts.RepositoryName, "repository-name", "", "The repository name")
	cmd.PersistentFlags().StringVar(&createOpts.TargetBranch, "target-branch", "", "The target branch of the repository  event to handle (eg: main, nightly)")
	cmd.PersistentFlags().StringVar(&createOpts.EventType, "event-type", "", "The event type of the repository event to handle (eg: pull_request, push)")
	cmd.PersistentFlags().StringVar(&createOpts.TargetURL, "repository-url", "", "The repository URL from where the event will come from")
	cmd.PersistentFlags().StringVar(&createOpts.TargetNamespace, "target-namespace", "", "The target namespace where the runs will be created")

	return cmd
}

// getGitInfo try to detect the current remote for this URL
func getGitInfo() (string, string) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return "", ""
	}
	cmd := exec.Command(gitPath, "remote", "get-url", "origin")
	bgitUrl, err := cmd.Output()
	if err != nil {
		cmd := exec.Command(gitPath, "remote", "get-url", "upstream")
		bgitUrl, err = cmd.Output()
		if err != nil {
			return "", ""
		}
	}
	gitURL := strings.TrimSpace(string(bgitUrl))
	if strings.HasPrefix(gitURL, "git@") {
		sp := strings.Split(gitURL, ":")
		prefix := strings.Replace(sp[0], "git@", "https://", -1)
		gitURL = fmt.Sprintf("%s/%s", prefix, strings.Join(sp[1:], ":"))
	}
	if strings.HasSuffix(gitURL, ".git") {
		gitURL = strings.TrimSuffix(gitURL, ".git")
	}

	cmd = exec.Command(gitPath, "rev-parse", "--show-toplevel")
	brootdir, err := cmd.Output()
	if err != nil {
		return "", ""
	}
	return gitURL, strings.TrimSpace(string(brootdir))
}

// askToCreateSimplePipeline will try to create a basic pipeline in tekton
// directory.
func askToCreateSimplePipeline(gitRoot string, opts CreateOptions) error {
	var repo string
	err := opts.CLIOpts.Ask([]*survey.Question{{
		Prompt: &survey.Select{
			Options: []string{"Yes", "No"},
			Default: "Yes",
			Message: fmt.Sprintf("Would you like to create a basic PipelineRun in your repo?"),
		},
	}}, &repo)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Join(gitRoot, ".tekton"), 0755); err != nil {
		return err
	}
	fpath := filepath.Join(gitRoot, ".tekton", fmt.Sprintf("%s.yaml", opts.EventType))
	tmpl := fmt.Sprintf(`---
apiVersion: tekton.dev/v1beta1
kind: PipelineRun
metadata:
  name: %s
  annotations:
    pipelinesascode.tekton.dev/on-target-branch: "[%s]"
    pipelinesascode.tekton.dev/on-event: "[%s]"
    pipelinesascode.tekton.dev/task: "[git-clone]"
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
      - name: git-clone
        taskRef:
          name: git-clone
        params:
          - name: url
            value: $(params.repo_url)
          - name: revision
            value: $(params.revision)
        workspaces:
          - name: output
            workspace: source
      - name: task
        taskSpec:
          steps:
            - name: task
              image: registry.access.redhat.com/ubi8/ubi-micro:8.4
              script : |
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
`, opts.RepositoryName, opts.TargetBranch, opts.EventType)
	err = ioutil.WriteFile(fpath, []byte(tmpl), 0644)
	if err != nil {
		return err
	}

	cs := opts.IOStreams.ColorScheme()
	fmt.Fprintf(opts.IOStreams.ErrOut, "%s A basic template has been created in %s, feel free to customize it.\n",
		cs.SuccessIcon(),
		cs.Bold(fpath),
	)
	fmt.Fprintf(opts.IOStreams.ErrOut, "%s You can test your pipeline manually with :.\n", cs.InfoIcon())
	fmt.Fprintf(opts.IOStreams.ErrOut, "tkn-pac resolve -f --generateName \\\n"+
		"     --params revision=%s --params repo_url=\"%s\" \\\n      -f %s\n", opts.TargetBranch, opts.TargetURL, fpath)

	return nil
}

// create ...
func create(ctx context.Context, opts CreateOptions) error {
	var qs []*survey.Question
	detectedGitRemoteURL, gitRoot := getGitInfo()

	if opts.TargetNamespace == "" {
		qs = append(qs, &survey.Question{
			Name:   "TargetNamespace",
			Prompt: &survey.Input{Message: fmt.Sprintf("Enter the target namespace (default: %s):", opts.CurrentNS)},
		})
	}
	if opts.TargetURL == "" {
		prompt := "Enter the target url: "
		if detectedGitRemoteURL != "" {
			prompt = fmt.Sprintf("Enter the target url (default: %s): ", detectedGitRemoteURL)
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

	err := opts.CLIOpts.Ask(qs, &opts)
	if err != nil {
		return err
	}
	if opts.TargetNamespace == "" {
		opts.TargetNamespace = opts.CurrentNS
	}
	if opts.TargetURL == "" && detectedGitRemoteURL != "" {
		opts.TargetURL = detectedGitRemoteURL
	} else if opts.TargetURL == "" {
		return fmt.Errorf("we didn't get a target URL")
	}
	if opts.TargetBranch == "" {
		opts.TargetBranch = "main"
	}
	if opts.RepositoryName == "" {
		s, err := ui.GetRepoOwnerFromGHURL(opts.TargetURL)
		generatedNS := fmt.Sprintf("%s-%s", filepath.Base(s), strings.ReplaceAll(opts.EventType, "_", "-"))
		prompt := fmt.Sprintf("Set a name for this resource (default: %s):", generatedNS)
		if err != nil {
			prompt = "Set a name for this resource:"
			generatedNS = ""
		}

		var repo string
		err = opts.CLIOpts.Ask([]*survey.Question{{
			Prompt: &survey.Input{Message: prompt},
		}}, &repo)
		if err != nil {
			return err
		}
		if repo == "" && generatedNS == "" {
			return fmt.Errorf("no name has been set")
		}
		if repo == "" {
			repo = generatedNS
		}
		opts.RepositoryName = repo
	}
	cs := opts.IOStreams.ColorScheme()
	if opts.TargetNamespace != opts.CurrentNS {
		_, err := opts.Clients.Kube.CoreV1().Namespaces().Get(ctx, opts.TargetNamespace, metav1.GetOptions{})
		if err != nil {
			fmt.Fprintf(opts.IOStreams.ErrOut, "%s Namespace %s is not created yet\n",
				cs.WarningIcon(),
				opts.TargetNamespace,
			)
			var ans string
			err := opts.CLIOpts.Ask([]*survey.Question{{
				Prompt: &survey.Select{
					Options: []string{"Yes", "No"},
					Default: "Yes",
					Message: fmt.Sprintf("Would you like me to create the namespace %s?", opts.TargetNamespace),
				},
			}}, &ans)
			if err != nil {
				return err
			}
			if ans != "Yes" {
				return fmt.Errorf("you need to create the target namespace..")
			}
			_, err = opts.Clients.Kube.CoreV1().Namespaces().Create(ctx,
				&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: opts.TargetNamespace}},
				metav1.CreateOptions{})
			if err != nil {
				return err
			}
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
	fmt.Fprintf(opts.IOStreams.ErrOut, "%s Repository %s has been created in %s namespace\n",
		cs.SuccessIconWithColor(cs.Green),
		opts.RepositoryName,
		opts.TargetNamespace,
	)

	if _, err = os.Stat(filepath.Join(gitRoot, ".tekton")); err != nil {
		if os.IsNotExist(err) {
			if err := askToCreateSimplePipeline(gitRoot, opts); err != nil {
				return err
			}
		}
	}
	fmt.Fprintf(opts.IOStreams.ErrOut, "%s Don't forget to install the GitHub application into your repo %s\n",
		cs.InfoIcon(),
		opts.TargetURL,
	)
	fmt.Fprintf(opts.IOStreams.ErrOut, "%s and we are done! enjoy :)))\n", cs.SuccessIcon())

	return nil
}
