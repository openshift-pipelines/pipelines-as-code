package repository

import (
	"context"
	"fmt"
	"strings"
	"text/tabwriter"
	"text/template"

	"github.com/AlecAivazis/survey/v2"
	"github.com/jonboulle/clockwork"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli/completion"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli/ui"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/flags"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/pipelineascode"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	promptStr        = "Choose a repository"
	describeTemplate = `{{ $.ColorScheme.Bold "Name" }}:	{{.Repository.Name}}
{{ $.ColorScheme.Bold "Namespace" }}:	{{.Repository.Namespace}}
{{ $.ColorScheme.Bold "URL" }}:	{{.Repository.Spec.URL}}
{{ $.ColorScheme.Bold "Event Type" }}:	{{formatEventType .Repository.Spec.EventType}}
{{ $.ColorScheme.Bold "Target Branch" }}:	{{.Repository.Spec.Branch}}

{{- if eq (len .Repository.Status) 0 }}

{{ $.ColorScheme.Dimmed "No runs has started."}}
{{- else }}

{{- $status := (index .Statuses 0) }}

{{ $.ColorScheme.Underline "Last Run:" }}

{{ $.ColorScheme.Bold "PipelineRun" }}:	{{ $status.PipelineRunName }}
{{ $.ColorScheme.Bold "Status" }}:	{{ $.ColorScheme.ColorStatus (index $status.Status.Conditions 0).Reason  }}
{{ $.ColorScheme.Bold "Commit" }}:	{{.Repository.Spec.URL}}/commit/{{ shortSHA $status.SHA }}
{{ $.ColorScheme.Bold "Commit Title" }}:	{{ $status.Title }}
{{ $.ColorScheme.Bold "StartTime" }}:	{{ formatTime $status.StartTime $.Clock }}
{{ $.ColorScheme.Bold "Duration" }}:	{{ formatDuration $status.StartTime $status.CompletionTime }}

{{- if gt (len .Repository.Status) 1 }}

{{ $.ColorScheme.Underline "Other Runs:" }}

{{ $.ColorScheme.BulletSpace }}PIPELINERUN	SHA	START_TIME	DURATION	STATUS

{{- range $i, $st := (slice .Statuses 1 (len .Repository.Status)) }}
{{ $.ColorScheme.Bullet }}{{ formatStatus $st $.ColorScheme $.Clock }}
{{- end }}
{{- end }}
{{- end }}

`
)

func formatStatus(status v1alpha1.RepositoryRunStatus, cs *ui.ColorScheme, c clockwork.Clock) string {
	return fmt.Sprintf("%s\t%s\t%s\t%s\t%s",
		status.PipelineRunName,
		ui.ShortSHA(*status.SHA),
		pipelineascode.Age(status.StartTime, c),
		pipelineascode.Duration(status.StartTime, status.CompletionTime),
		cs.ColorStatus(status.Status.Conditions[0].Reason))
}

func askRepo(ctx context.Context, cs *cli.Clients, opts *flags.CliOpts, namespace string) (*v1alpha1.Repository, error) {
	repositories, err := cs.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	if len(repositories.Items) == 0 {
		return nil, fmt.Errorf("No repo found")
	}
	if len(repositories.Items) == 1 {
		return &repositories.Items[0], nil
	}

	var allRepositories []string
	for _, repository := range repositories.Items {
		repoOwner, err := ui.GetRepoOwnerFromGHURL(repository.Spec.URL)
		if err != nil {
			return nil, err
		}
		allRepositories = append(allRepositories,
			fmt.Sprintf("%s - %s on %s",
				repository.GetName(),
				repoOwner, repository.Spec.EventType))
	}

	qs := []*survey.Question{
		{
			Name: "repository",
			Prompt: &survey.Select{
				Message: "Select a repository",
				Options: allRepositories,
			},
		},
	}
	var replyString string
	err = opts.Ask(qs, &replyString)
	if err != nil {
		return nil, err
	}

	if replyString == "" {
		return nil, fmt.Errorf("you need to choose a repository")
	}
	replyName := strings.Fields(replyString)[0]

	for _, repository := range repositories.Items {
		if repository.GetName() == replyName {
			return &repository, nil
		}
	}

	return nil, fmt.Errorf("cannot match repository???")
}

func DescribeCommand(p cli.Params) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "describe",
		Aliases: []string{"desc"},
		Short:   "Describe a repository",
		Annotations: map[string]string{
			"commandType": "main",
		},
		ValidArgsFunction: completion.ParentCompletion,
		RunE: func(cmd *cobra.Command, args []string) error {
			var repoName string
			opts, err := flags.NewCliOptions(cmd)
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
			cs, err := p.Clients()
			if err != nil {
				return err
			}
			ioStreams := ui.NewIOStreams()
			ioStreams.SetColorEnabled(!opts.NoColoring)
			return describe(ctx, cs, clock, opts, ioStreams, p.GetNamespace(), repoName)
		},
	}
	cmd.Flags().StringP(
		namespaceFlag, "n", "", "If present, the namespace scope for this CLI request")

	_ = cmd.RegisterFlagCompletionFunc(namespaceFlag,
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return completion.BaseCompletion(namespaceFlag, args)
		},
	)
	return cmd
}

func describe(ctx context.Context, cs *cli.Clients, clock clockwork.Clock, opts *flags.CliOpts,
	ioStreams *ui.IOStreams, namespace, repoName string) error {
	var repository *v1alpha1.Repository
	var err error

	if opts.Namespace != "" {
		namespace = opts.Namespace
	}

	if repoName != "" {
		repository, err = cs.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(namespace).Get(ctx,
			repoName, metav1.GetOptions{})
		if err != nil {
			return err
		}
	} else {
		repository, err = askRepo(ctx, cs, opts, namespace)
		if err != nil {
			return err
		}
	}

	colorScheme := ioStreams.ColorScheme()

	funcMap := template.FuncMap{
		"formatStatus":    formatStatus,
		"formatEventType": ui.CamelCasit,
		"formatDuration":  pipelineascode.Duration,
		"formatTime":      pipelineascode.Age,
		"shortSHA":        ui.ShortSHA,
	}

	data := struct {
		Repository  *v1alpha1.Repository
		Statuses    []v1alpha1.RepositoryRunStatus
		ColorScheme *ui.ColorScheme
		Clock       clockwork.Clock
	}{
		Repository:  repository,
		Statuses:    pipelineascode.SortedStatus(repository.Status),
		ColorScheme: colorScheme,
		Clock:       clock,
	}

	w := tabwriter.NewWriter(ioStreams.Out, 0, 5, 3, ' ', tabwriter.TabIndent)
	t := template.Must(template.New("Describe Repository").Funcs(funcMap).Parse(describeTemplate))

	if err := t.Execute(w, data); err != nil {
		return err
	}

	return w.Flush()
}
