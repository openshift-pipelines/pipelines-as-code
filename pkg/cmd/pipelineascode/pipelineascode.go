package pipelineascode

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/flags"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	pacpkg "github.com/openshift-pipelines/pipelines-as-code/pkg/pipelineascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"
	"github.com/spf13/cobra"
)

const (
	defaultURL = "https://giphy.com/explore/cat"
)

func Command(p cli.Params) *cobra.Command {
	opts := &pacpkg.Options{}
	cmd := &cobra.Command{
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
				return fmt.Errorf("token option is not set properly")
			}

			return runWrap(p, opts)
		},
	}

	flags.AddPacOptions(cmd)

	cmd.Flags().StringVarP(&opts.RunInfo.SHA, "webhook-sha", "", os.Getenv("PAC_SHA"), "SHA to test")
	cmd.Flags().StringVarP(&opts.RunInfo.Owner, "webhook-owner", "", os.Getenv("PAC_OWNER"), "Owner of the of the repository to test")
	cmd.Flags().StringVarP(&opts.RunInfo.Repository, "webhook-repository", "", os.Getenv("PAC_REPOSITORY_NAME"), "Repository Name of the repository to test")
	cmd.Flags().StringVarP(&opts.RunInfo.DefaultBranch, "webhook-defaultbranch", "", os.Getenv("PAC_DEFAULTBRANCH"), "DefaultBranch of the repository to test")
	cmd.Flags().StringVarP(&opts.RunInfo.Branch, "webhook-target-branch", "", os.Getenv("PAC_BRANCH"), "Target branch of the repository to test")
	cmd.Flags().StringVarP(&opts.RunInfo.URL, "webhook-url", "", os.Getenv("PAC_URL"), "URL of the repository to test")

	cmd.Flags().StringVarP(&opts.Payload, "payload", "", os.Getenv("PAC_PAYLOAD"), "The payload from webhook")
	cmd.Flags().StringVarP(&opts.PayloadFile, "payload-file", "", os.Getenv("PAC_PAYLOAD_FILE"), "A file which contains the webhook payload")
	return cmd
}

func getRunInfoFromArgsOrPayload(cs *cli.Clients, payload string, runinfo *webvcs.RunInfo) (*webvcs.RunInfo, error) {
	if err := runinfo.Check(); err == nil {
		return runinfo, err
	} else if payload == "" {
		return &webvcs.RunInfo{}, fmt.Errorf("no payload or not enough params set properly")
	}

	payloadinfo, err := cs.GithubClient.ParsePayload(cs.Log, payload)
	if err != nil {
		return &webvcs.RunInfo{}, err
	}

	if err := payloadinfo.Check(); err != nil {
		return &webvcs.RunInfo{}, fmt.Errorf("invalid Payload, missing some values : %+v", runinfo)
	}

	return payloadinfo, nil
}

// Wrap around a Run, create a CheckStatusID if there is a failure.
func runWrap(p cli.Params, opts *pacpkg.Options) error {
	cs, err := p.Clients()
	if err != nil {
		return err
	}

	if opts.PayloadFile != "" {
		_, err := os.Stat(opts.PayloadFile)
		if err != nil {
			return err
		}

		b, err := ioutil.ReadFile(opts.PayloadFile)
		if err != nil {
			return err
		}
		opts.Payload = string(b)
	}

	runinfo, err := getRunInfoFromArgsOrPayload(cs, opts.Payload, &opts.RunInfo)
	if err != nil {
		return err
	}

	kinteract, err := kubeinteraction.NewKubernetesInteraction(cs)
	if err != nil {
		return err
	}

	// Get webconsole url as soon as possible to have a link to click there
	url, err := kinteract.GetConsoleUI("", "")
	if err != nil {
		cs.Log.Error(err)
		url = defaultURL
	}
	runinfo.WebConsoleURL = url

	err = pacpkg.Run(cs, kinteract, runinfo)
	if err != nil && !strings.Contains(err.Error(), "403 Resource not accessible by integration") {
		_, _ = cs.GithubClient.CreateStatus(runinfo, "completed", "failure",
			fmt.Sprintf("There was an issue validating the commit: %q", err),
			runinfo.WebConsoleURL)
	}
	return err
}
