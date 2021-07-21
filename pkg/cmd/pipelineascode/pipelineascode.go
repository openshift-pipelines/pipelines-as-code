package pipelineascode

import (
	"context"
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
	defaultURL             = "https://giphy.com/explore/cat"
	defaultApplicationName = "Pipelines as Code CI"
)

func Command(p cli.Params) *cobra.Command {
	opts := &pacpkg.Options{}
	cmd := &cobra.Command{
		Use:   "pipelines-as-code",
		Short: "Pipelines as code Run",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return flags.GetWebCVSOptions(p, cmd)
		},
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			token, err := cmd.LocalFlags().GetString("token")
			if token == "" || err != nil {
				return fmt.Errorf("token option is not set properly")
			}
			cs, err := p.Clients()
			if err != nil {
				return err
			}
			kinteract, err := kubeinteraction.NewKubernetesInteraction(cs)
			if err != nil {
				return err
			}

			ctx := context.Background()
			return runWrap(ctx, opts, cs, kinteract)
		},
	}

	flags.AddPacOptions(cmd)
	flags.AddWebCVSOptions(cmd)

	cmd.Flags().StringVarP(&opts.RunInfo.EventType, "webhook-type", "", os.Getenv("PAC_EVENT_TYPE"), "Payload event type as set from Github (ie: X-GitHub-Event header)")
	cmd.Flags().StringVarP(&opts.RunInfo.TriggerTarget, "trigger-target", "", os.Getenv("PAC_TRIGGER_TARGET"), "The trigger target from where this event comes from")
	cmd.Flags().StringVarP(&opts.PayloadFile, "payload-file", "", os.Getenv("PAC_PAYLOAD_FILE"), "A file containing the webhook payload")
	applicationName := os.Getenv("PAC_APPLICATION_NAME")
	if applicationName == "" {
		applicationName = defaultApplicationName
	}

	cmd.Flags().StringVar(&opts.RunInfo.ApplicationName,
		"application-name", applicationName,
		"The name of the application.")
	return cmd
}

func parsePayload(ctx context.Context, cs *cli.Clients, opts *pacpkg.Options) (*webvcs.RunInfo, error) {
	if opts.PayloadFile == "" {
		return nil, fmt.Errorf("no payload file has been passed")
	}
	_, err := os.Stat(opts.PayloadFile)
	if err != nil {
		return nil, err
	}

	payloadB, err := ioutil.ReadFile(opts.PayloadFile)
	if err != nil {
		return nil, err
	}

	payloadinfo, err := cs.GithubClient.ParsePayload(ctx, cs.Log, opts.RunInfo.EventType,
		opts.RunInfo.TriggerTarget, string(payloadB))
	if err != nil {
		return &webvcs.RunInfo{}, err
	}
	payloadinfo.ApplicationName = opts.RunInfo.ApplicationName

	if err := payloadinfo.Check(); err != nil {
		return &webvcs.RunInfo{}, fmt.Errorf("invalid Payload, missing some values : %+v", payloadinfo)
	}

	return payloadinfo, nil
}

// Wrap around a Run, create a CheckStatusID if there is a failure.
func runWrap(ctx context.Context, opts *pacpkg.Options, cs *cli.Clients, kinteract cli.KubeInteractionIntf) error {
	runinfo, err := parsePayload(ctx, cs, opts)
	if err != nil {
		return err
	}

	// Get webconsole url as soon as possible to have a link to click there
	url, err := kinteract.GetConsoleUI(ctx, "", "")
	if err != nil {
		cs.Log.Error(err)
		url = defaultURL
	}
	runinfo.LogURL = url

	err = pacpkg.Run(ctx, cs, kinteract, runinfo)
	if err != nil {
		if runinfo.CheckRunID != nil && !strings.Contains(err.Error(), "403 Resource not accessible by integration") {
			_, _ = cs.GithubClient.CreateStatus(ctx, runinfo, "completed", "failure",
				fmt.Sprintf("There was an issue validating the commit: %q", err),
				runinfo.LogURL)
		} else {
			cs.Log.Debug("There was an error: %s", err.Error())
		}
	}
	return err
}
