package describe

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"regexp"
	"text/tabwriter"
	"text/template"

	"github.com/jonboulle/clockwork"
	"github.com/juju/ansiterm"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli/prompt"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli/status"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/tknpac/completion"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/consoleui"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/formatting"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	namespaceFlag   = "namespace"
	useRealTimeFlag = "use-realtime"
)

//go:embed templates/describe.tmpl
var describeTemplate string

func formatError(cs *cli.ColorScheme, log string) string {
	n := status.ErorrRE.ReplaceAllString(log, cs.RedBold("$0"))
	// add two space to every characters at beginning of line in string
	n = regexp.MustCompile(`(?m)^`).ReplaceAllString(n, "  ")
	return n
}

func formatStatus(status v1alpha1.RepositoryRunStatus, cs *cli.ColorScheme, c clockwork.Clock) string {
	return fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s\t%s",
		cs.ColorStatus(status.Status.Conditions[0].Reason),
		*status.EventType,
		*status.TargetBranch,
		cs.HyperLink(formatting.ShortSHA(*status.SHA), *status.SHAURL),
		formatting.Age(status.StartTime, c),
		formatting.PRDuration(status),
		cs.HyperLink(status.PipelineRunName, *status.LogURL))
}

func Root(run *params.Run, ioStreams *cli.IOStreams) *cobra.Command {
	var useRealTime bool
	cmd := &cobra.Command{
		Use:     "describe",
		Aliases: []string{"desc"},
		Short:   "Describe a repository",
		Annotations: map[string]string{
			"commandType": "main",
		},
		ValidArgsFunction: completion.ParentCompletion,
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			var repoName string
			opts := cli.NewCliOptions(cmd)

			opts.UseRealTime, err = cmd.Flags().GetBool(useRealTimeFlag)
			if err != nil {
				return err
			}

			opts.Namespace, err = cmd.Flags().GetString(namespaceFlag)
			if err != nil {
				return err
			}

			if len(args) > 0 {
				repoName = args[0]
			}

			ctx := context.Background()
			clock := clockwork.NewRealClock()
			err = run.Clients.NewClients(ctx, &run.Info)
			if err != nil {
				return err
			}

			// The only way to know the tekton dashboard url is if the user specify it because we are not supposed to have access to the configmap.
			// so let the user specify a env variable to implicitly set tekton dashboard
			if os.Getenv("TEKTON_DASHBOARD_URL") != "" {
				run.Clients.ConsoleUI = &consoleui.TektonDashboard{BaseURL: os.Getenv("TEKTON_DASHBOARD_URL")}
			}

			return describe(ctx, run, clock, opts, ioStreams, repoName)
		},
	}

	cmd.Flags().StringP(
		namespaceFlag, "n", "", "If present, the namespace scope for this CLI request")

	cmd.PersistentFlags().BoolVarP(&useRealTime, useRealTimeFlag, "", false,
		"display the time as RFC3339 instead of a relative time")
	_ = cmd.RegisterFlagCompletionFunc(namespaceFlag,
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return completion.BaseCompletion(namespaceFlag, args)
		},
	)
	return cmd
}

func describe(ctx context.Context, cs *params.Run, clock clockwork.Clock, opts *cli.PacCliOpts, ioStreams *cli.IOStreams, repoName string) error {
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

	colorScheme := ioStreams.ColorScheme()

	funcMap := template.FuncMap{
		"formatError":     formatError,
		"formatStatus":    formatStatus,
		"formatEventType": formatting.CamelCasit,
		"formatDuration":  formatting.PRDuration,
		"formatTime":      formatting.Age,
		"sanitizeBranch":  formatting.SanitizeBranch,
		"shortSHA":        formatting.ShortSHA,
	}

	data := struct {
		Repository  *v1alpha1.Repository
		Statuses    []v1alpha1.RepositoryRunStatus
		ColorScheme *cli.ColorScheme
		Clock       clockwork.Clock
		Opts        *cli.PacCliOpts
	}{
		Repository:  repository,
		Statuses:    status.MixLivePRandRepoStatus(ctx, cs, *repository),
		ColorScheme: colorScheme,
		Clock:       clock,
		Opts:        opts,
	}
	w := ansiterm.NewTabWriter(ioStreams.Out, 0, 5, 3, ' ', tabwriter.TabIndent)
	t := template.Must(template.New("Describe Repository").Funcs(funcMap).Parse(describeTemplate))

	if err := t.Execute(w, data); err != nil {
		return err
	}

	return w.Flush()
}
