package logs

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli/prompt"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/tknpac/completion"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/consoleui"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const longhelp = `

logs - show the PipelineRun logs attached to a Repository

tkn pac logs will get the logs of a PipelineRun belonging to a Repository.

the PipelineRun needs to exist on the kubernetes cluster to be able to display the logs.`

var (
	namespaceFlag = "namespace"
	shiftFlag     = "shift"
)

func Command(run *params.Run, ioStreams *cli.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs",
		Long:  longhelp,
		Short: "Display the PipelineRun logs from a Repository",
		Annotations: map[string]string{
			"commandType": "main",
		},
		ValidArgsFunction: completion.ParentCompletion,
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			var repoName string
			opts := cli.NewCliOptions(cmd)

			opts.Namespace, err = cmd.Flags().GetString(namespaceFlag)
			if err != nil {
				return err
			}

			if len(args) > 0 {
				repoName = args[0]
			}

			ctx := context.Background()
			err = run.Clients.NewClients(ctx, &run.Info)
			if err != nil {
				return err
			}

			// The only way to know the tekton dashboard url is if the user specify it because we are not supposed to have access to the configmap.
			// so let the user specify a env variable to implicitly set tekton dashboard
			if os.Getenv("TEKTON_DASHBOARD_URL") != "" {
				run.Clients.ConsoleUI = &consoleui.TektonDashboard{BaseURL: os.Getenv("TEKTON_DASHBOARD_URL")}
			}

			shift, err := cmd.Flags().GetInt(shiftFlag)
			if err != nil {
				return err
			}

			return log(ctx, run, opts, ioStreams, repoName, shift)
		},
	}

	cmd.Flags().StringP(
		namespaceFlag, "n", "", "If present, the namespace scope for this CLI request")

	cmd.Flags().IntP(
		shiftFlag, "s", 1, "Show the last N number of Repository if it exist")

	return cmd
}

func getTknPath() (string, error) {
	fname, err := exec.LookPath("tkn")
	if err != nil {
		return "", err
	}

	return filepath.Abs(fname)
}

func log(ctx context.Context, cs *params.Run, opts *cli.PacCliOpts, ioStreams *cli.IOStreams, repoName string, shift int) error {
	var repository *v1alpha1.Repository
	var err error

	if opts.Namespace != "" {
		cs.Info.Kube.Namespace = opts.Namespace
	}

	if repoName != "" {
		repository, err = cs.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(cs.Info.Kube.Namespace).Get(ctx,
			repoName, metav1.GetOptions{})
		if err != nil {
			return err
		}
	} else {
		repository, err = prompt.SelectRepo(ctx, cs, cs.Info.Kube.Namespace)
		if err != nil {
			return err
		}
	}

	tknpac, err := getTknPath()
	if err != nil {
		return err
	}
	if len(repository.Status) == 0 {
		return fmt.Errorf("no status on repository")
	}
	prName := repository.Status[len(repository.Status)-shift].PipelineRunName
	pr, err := cs.Clients.Tekton.TektonV1beta1().PipelineRuns(cs.Info.Kube.Namespace).Get(ctx, prName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	fmt.Fprintf(ioStreams.Out, "Showing logs from Repository: %s PR: %s\n", repository.GetName(), prName)

	// if we have found the plugin then sysexec it by replacing current process.
	if err := syscall.Exec(tknpac, []string{tknpac, "pr", "logs", "-n", cs.Info.Kube.Namespace, pr.GetName()}, os.Environ()); err != nil {
		fmt.Fprintf(os.Stderr, "Command finished with error: %v", err)
		os.Exit(127)
	}
	return nil
}
