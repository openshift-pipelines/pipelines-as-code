package pipelineascode

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/pipelineascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs/github"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/spf13/cobra"
)

const (
	defaultURL = "https://giphy.com/explore/cat"
)

func Command(cs *params.Run) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "pipelines-as-code",
		Short:        "Pipelines as code Run",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := cs.Clients.NewClients(&cs.Info)
			if err != nil {
				return err
			}

			kinteract, err := kubeinteraction.NewKubernetesInteraction(cs)
			if err != nil {
				return err
			}
			ctx := context.Background()
			vcsintf, err := getVCS(cs.Info.Pac)
			if err != nil {
				return err
			}
			return runWrap(ctx, cs, vcsintf, kinteract)
		},
	}

	cs.Info.Pac.AddFlags(cmd)
	cs.Info.Kube.AddFlags(cmd)

	cmd.Flags().StringVarP(&cs.Info.Event.EventType, "webhook-type", "", os.Getenv("PAC_EVENT_TYPE"), "Payload event type as set from Github (ie: X-GitHub-Event header)")
	cmd.Flags().StringVarP(&cs.Info.Event.TriggerTarget, "trigger-target", "", os.Getenv("PAC_TRIGGER_TARGET"), "The trigger target from where this event comes from")

	return cmd
}

func getPayloadFromFile(opts info.PacOpts) (string, error) {
	if opts.PayloadFile == "" {
		return "", fmt.Errorf("no payload file has been passed")
	}
	_, err := os.Stat(opts.PayloadFile)
	if err != nil {
		return "", err
	}

	payloadB, err := ioutil.ReadFile(opts.PayloadFile)
	return string(payloadB), err
}

func getVCS(pacopts info.PacOpts) (webvcs.Interface, error) {
	switch pacopts.VCSType {
	case "github":
		g := &github.VCS{}
		return g, nil
	default:
		return nil, fmt.Errorf("no supported VCS is detected")
	}
}

func runWrap(ctx context.Context, cs *params.Run, vcx webvcs.Interface, kinteract kubeinteraction.Interface) error {
	var err error

	cs.Info.Pac.LogURL, err = kinteract.GetConsoleUI(ctx, "", "")
	if err != nil {
		cs.Clients.Log.Warn("could not detect a Console URL, skipping")
		cs.Info.Pac.LogURL = defaultURL
	}

	// If we already have the Token (ie: github apps) set as soon as possible the client,
	// There is more things supported when we already have a github apps and some that are not
	// (ie: /ok-to-test or /rerequest)
	if cs.Info.Pac.VCSToken != "" {
		vcx.SetClient(ctx, cs.Info.Pac)
	}

	payload, err := getPayloadFromFile(cs.Info.Pac)
	if err != nil {
		return err
	}

	cs.Info.Event, err = vcx.ParsePayload(ctx, cs, payload)
	if err != nil {
		return err
	}

	err = pipelineascode.Run(ctx, cs, vcx, kinteract)
	if err != nil {
		createStatusErr := vcx.CreateStatus(ctx, cs.Info.Event, cs.Info.Pac, webvcs.StatusOpts{
			Status:     "completed",
			Conclusion: "failure",
			Text:       fmt.Sprintf("There was an issue validating the commit: %q", err),
			DetailsURL: "",
		})
		if createStatusErr != nil {
			cs.Clients.Log.Errorf("Cannot create status: %s %s", err, createStatusErr)
		}
	}
	return err
}
