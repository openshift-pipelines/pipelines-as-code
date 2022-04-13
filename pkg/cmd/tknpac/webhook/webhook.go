package webhook

import (
	"context"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/tknpac/bootstrap"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
)

type Webhook struct {
	RepositoryURL  string
	PACNamespace   string
	ControllerURL  string
	ProviderAPIURL string
}

func (w Webhook) Install(ctx context.Context, run *params.Run) error {
	// figure out pac installation namespace
	installationNS, err := bootstrap.DetectPacInstallation(ctx, w.PACNamespace, run)
	if err != nil {
		return err
	}

	// check if any other provider is already configured
	pacInfo, err := info.GetPACInfo(ctx, run, installationNS)
	if err != nil {
		return err
	}

	// if GitHub App is already configured then skip configuring webhook
	if pacInfo.Provider == provider.ProviderGitHubApp {
		return nil
	}

	route, _ := bootstrap.DetectOpenShiftRoute(ctx, run, w.PACNamespace)
	if route != "" {
		w.ControllerURL = route
	}

	return w.githubWebhook(ctx)
}
