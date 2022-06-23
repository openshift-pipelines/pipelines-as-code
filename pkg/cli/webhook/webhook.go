package webhook

import (
	"context"
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli/prompt"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/tknpac/bootstrap"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/tknpac/create"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/git"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	pacinfo "github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Interface interface {
	Run(context.Context, *Options) (*response, error)
}

type Options struct {
	Run                 *params.Run
	IOStreams           *cli.IOStreams
	PACNamespace        string
	RepositoryURL       string
	ProviderAPIURL      string
	ControllerURL       string
	repositoryName      string
	repositoryNamespace string
}

type response struct {
	UserName            string
	ControllerURL       string
	WebhookSecret       string
	PersonalAccessToken string
	APIURL              string
}

func (w *Options) Install(ctx context.Context, providerType string) error {
	if w.RepositoryURL == "" {
		q := "Please enter the Git repository url containing the pipelines: "
		if err := prompt.SurveyAskOne(&survey.Input{Message: q}, &w.RepositoryURL,
			survey.WithValidator(survey.Required)); err != nil {
			return err
		}
	}

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

	// check if info configmap has url then use that otherwise try to detect
	if pacInfo.ControllerURL != "" && w.ControllerURL == "" {
		w.ControllerURL = pacInfo.ControllerURL
	} else {
		w.ControllerURL, _ = bootstrap.DetectOpenShiftRoute(ctx, w.Run, w.PACNamespace)
	}

	var webhookProvider Interface
	switch providerType {
	case "github":
		webhookProvider = &gitHubConfig{IOStream: w.IOStreams}
	case "gitlab":
		webhookProvider = &gitLabConfig{IOStream: w.IOStreams}
	case "bitbucket-cloud":
		webhookProvider = &bitbucketCloudConfig{IOStream: w.IOStreams}
	default:
		return fmt.Errorf("invalid webhook provider")
	}

	response, err := webhookProvider.Run(ctx, w)
	if err != nil {
		return err
	}

	msg := "Would you like me to create the Repository CR for your git repository?"
	var createRepo bool
	if err := prompt.SurveyAskOne(&survey.Confirm{Message: msg, Default: true}, &createRepo); err != nil {
		return err
	}
	if !createRepo {
		fmt.Fprintln(w.IOStreams.Out, "✓ Skipping Repository creation")
		fmt.Fprintln(w.IOStreams.Out, "💡 Don't forget to create a secret with webhook secret and provider token & attaching in Repository.")
		return nil
	}

	repo := create.RepoOptions{
		Run: w.Run,
		Event: &pacinfo.Event{
			URL: w.RepositoryURL,
		},
		GitInfo: &git.Info{URL: w.RepositoryURL},
		Repository: &apipac.Repository{
			ObjectMeta: v1.ObjectMeta{},
		},
		IoStreams: w.IOStreams,
	}

	w.repositoryName, w.repositoryNamespace, err = repo.Create(ctx)
	if err != nil {
		return err
	}

	// create webhook secret in namespace where repository CR is created
	if err := w.createWebhookSecret(ctx, response); err != nil {
		return err
	}

	// update repo cr with webhook secret
	return w.updateRepositoryCR(ctx, response)
}
