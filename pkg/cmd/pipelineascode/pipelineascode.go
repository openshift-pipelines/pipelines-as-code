package pipelineascode

import (
	"fmt"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	pacpkg "github.com/openshift-pipelines/pipelines-as-code/pkg/pipelineascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"
	"github.com/spf13/cobra"
)

type pacOptions struct {
	githubToken   string
	githubPayload string
}

// InitParams initialises cli.Params based on flags defined in command
func InitParams(p cli.Params, cmd *cobra.Command) error {
	// ensure that the config is valid by creating a client
	if _, err := p.Clients(); err != nil {
		return err
	}
	return nil
}

func Command(p cli.Params) *cobra.Command {
	opts := &pacOptions{}
	var cmd = &cobra.Command{
		Use:   "run",
		Short: "Run pipelines as code",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := InitParams(p, cmd); err != nil {
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
			if opts.githubPayload == "" {
				return fmt.Errorf("github-payload needs to be set")
			}
			if opts.githubToken == "" {
				return fmt.Errorf("github-token needs to be set")
			}
			return run(p, opts)
		},
	}
	cmd.Flags().StringVarP(&opts.githubToken, "github-token", "", "", "Github Token used for operations")
	cmd.Flags().StringVarP(&opts.githubPayload, "github-payload", "", "", "Github Payload from webhook")
	return cmd
}

func run(p cli.Params, opts *pacOptions) error {
	gvcs := webvcs.NewGithubVCS(opts.githubToken)
	cs, err := p.Clients()
	if err != nil {
		return err
	}
	payload, err := gvcs.ParsePayload(opts.githubPayload)
	if err != nil {
		return err
	}
	op := pacpkg.PipelineAsCode{Client: cs.PipelineAsCode}
	repo, err := op.FilterBy(payload.URL, payload.Branch, "pull_request")
	if err != nil {
		return err
	}

	if repo.Spec.Namespace == "" {
		return nil
	}

	fmt.Println("Namespace for repository: " + payload.Owner + "/" + payload.Repository + " is " + repo.Spec.Namespace)
	return nil
}
