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
	tknPathFlag   = "tkn-path"
)

type logOption struct {
	cs        *params.Run
	opts      *cli.PacCliOpts
	ioStreams *cli.IOStreams
	repoName  string
	tknPath   string
	shift     int
}

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

			shift, err := cmd.Flags().GetInt(shiftFlag)
			if err != nil {
				return err
			}

			tknPath, err := cmd.Flags().GetString(tknPathFlag)
			if err != nil {
				return err
			}
			if tknPath == "" {
				if tknPath, err = getTknPath(); err != nil {
					return err
				}

				if tknPath == "" {
					return fmt.Errorf("cannot find tkn binary in Path")
				}
			}

			lopts := &logOption{
				cs:        run,
				opts:      opts,
				ioStreams: ioStreams,
				repoName:  repoName,
				shift:     shift,
				tknPath:   tknPath,
			}
			return log(ctx, lopts)
		},
	}

	cmd.Flags().StringP(
		tknPathFlag, "", "", "Path to the tkn binary (default to search for it in you $PATH)")

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

func log(ctx context.Context, lo *logOption) error {
	var repository *v1alpha1.Repository
	var err error

	if lo.opts.Namespace != "" {
		lo.cs.Info.Kube.Namespace = lo.opts.Namespace
	}

	if lo.repoName != "" {
		repository, err = lo.cs.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(lo.cs.Info.Kube.Namespace).Get(ctx,
			lo.repoName, metav1.GetOptions{})
		if err != nil {
			return err
		}
	} else {
		repository, err = prompt.SelectRepo(ctx, lo.cs, lo.cs.Info.Kube.Namespace)
		if err != nil {
			return err
		}
	}

	if len(repository.Status) == 0 {
		return fmt.Errorf("no status on repository")
	}
	shiftedinto := len(repository.Status) - lo.shift
	if shiftedinto < 0 {
		return fmt.Errorf("you have specified a shift of %d but we only have %d statuses", lo.shift, len(repository.Status))
	}
	prName := repository.Status[shiftedinto].PipelineRunName
	pr, err := lo.cs.Clients.Tekton.TektonV1beta1().PipelineRuns(lo.cs.Info.Kube.Namespace).Get(ctx, prName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	fmt.Fprintf(lo.ioStreams.Out, "Showing logs from Repository: %s PR: %s\n", repository.GetName(), prName)

	// if we have found the plugin then sysexec it by replacing current process.
	if err := syscall.Exec(lo.tknPath, []string{lo.tknPath, "pr", "logs", "-n", lo.cs.Info.Kube.Namespace, pr.GetName()}, os.Environ()); err != nil {
		fmt.Fprintf(os.Stderr, "Command finished with error: %v", err)
		os.Exit(127)
	}
	return nil
}
