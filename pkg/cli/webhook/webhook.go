package webhook

import (
	"context"
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli/prompt"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/tknpac/bootstrap"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
)

type Interface interface {
	GetName() string
	Run(context.Context, *Options) (*response, error)
}

type Options struct {
	Run                 *params.Run
	PACNamespace        string
	ControllerURL       string
	RepositoryURL       string
	ProviderAPIURL      string
	RepositoryName      string
	RepositoryNamespace string

	// github specific flag
	// allows configuring webhook if app is already configured
	GitHubWebhook bool
}

type response struct {
	ControllerURL       string
	WebhookSecret       string
	PersonalAccessToken string
}

func (w *Options) Install(ctx context.Context) error {
	// figure out pac installation namespace
	installationNS, err := bootstrap.DetectPacInstallation(ctx, w.PACNamespace, w.Run)
	if err != nil {
		return err
	}

	// fetch configmap to get controller url
	pacInfo, err := info.GetPACInfo(ctx, w.Run, installationNS)
	if err != nil {
		return err
	}

	// figure out which git provider from the Repo URL
	webhookProvider := detectProvider(w.RepositoryURL)

	if !w.GitHubWebhook && webhookProvider != nil {
		if webhookProvider.GetName() == provider.ProviderGitHubWebhook && pacInfo.Provider == provider.ProviderGitHubApp {
			// nolint
			fmt.Printf("✓ Skips configuring GitHub Webhook as GitHub App is already configured." +
				" Please pass --github-webhook flag to still configure it")
			return nil
		}
	}

	var msg string
	if webhookProvider != nil {
		msg = fmt.Sprintf("Would you like me to configure a %s Webhook for your repository? ",
			strings.TrimSuffix(webhookProvider.GetName(), "Webhook"))
	} else {
		msg = "Would you like me to configure a Webhook for your repository?"
	}

	var configureWebhook bool
	if err := prompt.SurveyAskOne(&survey.Confirm{Message: msg, Default: true}, &configureWebhook); err != nil {
		return err
	}
	if !configureWebhook {
		return nil
	}

	if webhookProvider == nil {
		if webhookProvider, err = askProvider(); webhookProvider == nil || err != nil {
			return err
		}
	}

	// check if info configmap has url then use that otherwise try to detec
	if pacInfo.ControllerURL != "" && w.ControllerURL == "" {
		w.ControllerURL = pacInfo.ControllerURL
	} else {
		w.ControllerURL, _ = bootstrap.DetectOpenShiftRoute(ctx, w.Run, w.PACNamespace)
	}

	response, err := webhookProvider.Run(ctx, w)
	if err != nil {
		return err
	}

	if err := w.askRepositoryCRDetails(); err != nil {
		return err
	}

	// create webhook secret in namespace where repository CR is created
	if err := w.createWebhookSecret(ctx, response); err != nil {
		return err
	}

	// update repo cr with webhook secret
	return w.updateRepositoryCR(ctx)
}

func askProvider() (Interface, error) {
	var answer string
	if err := survey.AskOne(&survey.Select{
		Message: "Please select the provider you wish to configure with your repository:",
		Options: []string{"GitHub", "GitLab"},
	}, &answer, survey.WithValidator(survey.Required)); err != nil {
		return nil, err
	}

	if answer == "GitHub" {
		return &gitHubConfig{}, nil
	} else if answer == "GitLab" {
		return &gitLabConfig{}, nil
	}
	return nil, nil
}

func detectProvider(url string) Interface {
	if strings.Contains(url, "github.com") {
		return &gitHubConfig{}
	} else if strings.Contains(url, "gitlab.com") {
		return &gitLabConfig{}
	}
	return nil
}

func (w *Options) askRepositoryCRDetails() error {
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
	return nil
}
