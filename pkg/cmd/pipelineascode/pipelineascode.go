package pipelineascode

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/flags"
	pacpkg "github.com/openshift-pipelines/pipelines-as-code/pkg/pipelineascode"
	"github.com/spf13/cobra"
)

func Command(p cli.Params) *cobra.Command {
	opts := &pacpkg.Options{}
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
			token, err := cmd.LocalFlags().GetString("token")
			if token == "" || err != nil {
				return fmt.Errorf("token option is not set properly.")
			}
			if opts.Payload == "" {
				return errors.New("payload needs to be set")
			}
			return runWrap(p, opts)
		},
	}

	flags.AddPacOptions(cmd)

	cmd.Flags().StringVarP(&opts.Payload, "payload", "", os.Getenv("PAS_PAYLOAD"), "The payload from webhook")
	return cmd
}

// Wrap around a Run, create a CheckStatusID if there is a failure.
func runWrap(p cli.Params, opts *pacpkg.Options) error {
	cs, err := p.Clients()
	if err != nil {
		return err
	}

	runInfo, err := cs.GithubClient.ParsePayload(opts.Payload)
	if err != nil {
		return err
	}

	err = pacpkg.Run(cs, runInfo)
	if err != nil {
		_, _ = cs.GithubClient.CreateStatus(runInfo, "completed", "failure",
			fmt.Sprintf("There was an issue validating the commit: %q", err),
			"https://tenor.com/search/sad-cat-gifs")
	}
	return err
}
