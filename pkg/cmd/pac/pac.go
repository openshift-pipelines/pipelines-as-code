package pac

import (
	"fmt"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/flags"
	pacpkg "github.com/openshift-pipelines/pipelines-as-code/pkg/pac"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"
	"github.com/spf13/cobra"
)

type pacOptions struct {
	github_token   string
	github_payload string
}

func Command(p cli.Params) *cobra.Command {
	opts := &pacOptions{}
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
			github_token, err := cmd.LocalFlags().GetString("github-token")
			if err != nil {
				return fmt.Errorf("github token is not set properly: %v", err)
			}
			github_payload, err := cmd.LocalFlags().GetString("github-payload")
			if err != nil {
				return fmt.Errorf("github payload is not set properly: %v", err)
			}
			gvcs := webvcs.NewGithubVCS(github_token)
			cs, err := p.Clients()
			if err != nil {
				return err
			}
			payload, err := gvcs.ParsePayload(github_payload)
			if err != nil {
				return err
			}
			op := pacpkg.Pac{Client: cs.Pac}
			repo, err := op.FilterBy(payload.URL, payload.Branch, "pull_request")
			if err != nil {
				return err
			}

			fmt.Println("Namespace for repository: " + payload.Owner + "/" + payload.Repository + " is " + repo.Spec.Namespace)
			return nil
		},
	}
	cmd.Flags().StringVarP(&opts.github_token, "github-token", "", "", "Github Token used for operations")
	cmd.Flags().StringVarP(&opts.github_payload, "github-payload", "", "", "Github Payload from webhook")
	return cmd
}
