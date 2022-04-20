package webhook

import (
	"context"

	"github.com/AlecAivazis/survey/v2"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli/prompt"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/tknpac/bootstrap"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
)

const (
	// nolint
	githubWebhookSecretName = "github-webhook-secret"
)

type Webhook struct {
	RepositoryURL       string
	PACNamespace        string
	ControllerURL       string
	ProviderAPIURL      string
	RepositoryName      string
	RepositoryNamespace string
}

type response struct {
	UserDeclined        bool
	ControllerURL       string
	WebhookSecret       string
	PersonalAccessToken string
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

	response, err := w.githubWebhook(ctx)
	if err != nil || response.UserDeclined {
		return err
	}

	if w.RepositoryName == "" {
		if err := prompt.SurveyAskOne(&survey.Input{
			Message: "Please enter the Repository CR Name to configure with webhook: ",
		}, &w.RepositoryName, survey.WithValidator(survey.Required)); err != nil {
			return err
		}
	}

	if w.RepositoryNamespace == "" {
		if err := prompt.SurveyAskOne(&survey.Input{
			Message: "Please enter the Repository CR Namespace to configure with webhook: ",
		}, &w.RepositoryNamespace, survey.WithValidator(survey.Required)); err != nil {
			return err
		}
	}

	// create webhook secret in namespace where repository CR is created
	if err := w.createWebhookSecret(ctx, run.Clients.Kube, githubWebhookSecretName, response); err != nil {
		return err
	}

	// update repo cr with webhook secret
	if err := w.updateRepositoryCR(ctx, run.Clients.PipelineAsCode, githubWebhookSecretName); err != nil {
		return err
	}

	// finally update info configmap with the provider configured so that
	// later if user runs bootstrap, they will know a provider is already
	// configured
	return info.UpdateInfoConfigMap(ctx, run, &info.Options{
		TargetNamespace: installationNS,
		ControllerURL:   response.ControllerURL,
		Provider:        provider.ProviderGitHubWebhook,
	})
}
