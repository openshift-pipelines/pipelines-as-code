package paac

import (
	"context"
	"fmt"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/flags"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Command(p cli.Params) *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "run",
		Short: "Run pipelines as code",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := flags.InitParams(p, cmd); err != nil {
				// this check allows tkn version to be run without
				// a kubeconfig so users can verify the tkn version
				noConfigErr := strings.Contains(err.Error(), "no configuration has been provided")
				if noConfigErr {
					return nil
				}
				return err
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Hello world")
			cs, err := p.Clients()
			t, err := cs.Kube.AppsV1().Deployments("openshift-pipelines").List(context.Background(), metav1.ListOptions{})
			if err != nil {
				return err
			}
			fmt.Printf("%+v\n", t)
			return nil
		},
	}
	return cmd
}
