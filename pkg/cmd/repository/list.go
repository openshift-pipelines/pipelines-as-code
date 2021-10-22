package repository

import (
	"context"
	"fmt"
	"text/tabwriter"

	"github.com/jonboulle/clockwork"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/completion"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/ui"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	header            = "NAME\tAGE\tURL"
	body              = "%s\t%s\t%s"
	allNamespacesFlag = "all-namespaces"
	namespaceFlag     = "namespace"
	noColorFlag       = "no-color"
)

func ListCommand(run *params.Run, ioStreams *ui.IOStreams) *cobra.Command {
	var noheaders, allNamespaces bool
	var selectors string

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List repositories",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := params.NewCliOptions(cmd)
			if err != nil {
				return err
			}

			opts.AllNameSpaces, err = cmd.Flags().GetBool(allNamespacesFlag)
			if err != nil {
				return err
			}
			opts.Namespace, err = cmd.Flags().GetString(namespaceFlag)
			if err != nil {
				return err
			}
			ioStreams.SetColorEnabled(!opts.NoColoring)

			err = run.Clients.NewClients(&run.Info)
			if err != nil {
				return err
			}
			ctx := context.Background()
			cw := clockwork.NewRealClock()
			return list(ctx, run, opts, ioStreams, cw, selectors, noheaders)
		},
	}

	cmd.PersistentFlags().BoolP(noColorFlag, "C", !ioStreams.ColorEnabled(), "disable coloring")
	cmd.PersistentFlags().BoolVarP(&allNamespaces, allNamespacesFlag, "A", false, "If present, "+
		"list the repository across all namespaces. Namespace in current context is ignored even if specified with"+
		" --namespace.")

	cmd.Flags().StringP(
		namespaceFlag, "n", "", "If present, the namespace scope for this CLI request")

	_ = cmd.RegisterFlagCompletionFunc(namespaceFlag,
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return completion.BaseCompletion(namespaceFlag, args)
		},
	)

	cmd.Flags().BoolVar(
		&noheaders, "no-headers", false, "don't print headers.")

	cmd.Flags().StringVarP(&selectors, "selectors", "l",
		"", "Selector (label query) to filter on, "+
			"supports '=', "+
			"'==',"+
			" and '!='.(e.g. -l key1=value1,key2=value2)")
	return cmd
}

func list(ctx context.Context, cs *params.Run, opts *params.PacCliOpts, ioStreams *ui.IOStreams, cw clockwork.Clock, selectors string, noheaders bool) error {
	if opts.Namespace != "" {
		cs.Info.Kube.Namespace = opts.Namespace
	}
	if opts.AllNameSpaces {
		cs.Info.Kube.Namespace = ""
	}

	lopt := metav1.ListOptions{LabelSelector: selectors}

	repositories, err := cs.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(cs.Info.Kube.Namespace).List(
		ctx, lopt)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(ioStreams.Out, 0, 5, 3, ' ', tabwriter.TabIndent)

	if !noheaders {
		_, _ = fmt.Fprint(w, header)
		if opts.AllNameSpaces {
			fmt.Fprint(w, "\tNAMESPACE")
		}
		fmt.Fprintln(w, "\tSTATUS")
	}
	for _, repository := range repositories.Items {
		fmt.Fprintf(w, body, repository.GetName(), ui.ShowLastAge(repository, cw), repository.Spec.URL)

		if opts.AllNameSpaces {
			fmt.Fprintf(w, "\t%s", repository.GetNamespace())
		}

		fmt.Fprintf(w, "\t%s", ui.ShowStatus(repository, ioStreams.ColorScheme()))
		fmt.Fprint(w, "\n")
	}

	w.Flush()
	return nil
}
