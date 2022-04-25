package webhook

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/AlecAivazis/survey/v2"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli/prompt"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"github.com/xanzy/go-gitlab"
)

type gitLabConfig struct {
	Client              *gitlab.Client
	controllerURL       string
	projectID           string
	webhookSecret       string
	personalAccessToken string
	APIURL              string
}

func (gl *gitLabConfig) GetName() string {
	return provider.ProviderGitLabWebhook
}

func (gl *gitLabConfig) Run(_ context.Context, opts *Options) (*response, error) {
	err := gl.askGLWebhookConfig(opts.ControllerURL)
	if err != nil {
		return nil, err
	}
	gl.APIURL = opts.ProviderAPIURL

	return &response{
		ControllerURL:       gl.controllerURL,
		PersonalAccessToken: gl.personalAccessToken,
		WebhookSecret:       gl.webhookSecret,
	}, gl.create()
}

func (gl *gitLabConfig) askGLWebhookConfig(controllerURL string) error {
	msg := "Please enter project ID for the repository you want to be configured :"
	if err := prompt.SurveyAskOne(&survey.Input{Message: msg}, &gl.projectID,
		survey.WithValidator(survey.Required)); err != nil {
		return err
	}

	if controllerURL == "" {
		if err := prompt.SurveyAskOne(&survey.Input{
			Message: "Please enter your controller public route URL: ",
		}, &gl.controllerURL, survey.WithValidator(survey.Required)); err != nil {
			return err
		}
	}

	if err := prompt.SurveyAskOne(&survey.Password{
		Message: "Please enter the secret to configure the webhook for payload validation: ",
	}, &gl.webhookSecret, survey.WithValidator(survey.Required)); err != nil {
		return err
	}

	// nolint:forbidigo
	fmt.Println("ℹ ️You now need to create a GitLab personal access token with `api` scope")
	// nolint:forbidigo
	fmt.Println("ℹ ️Go to this URL to generate one https://gitlab.com/-/profile/personal_access_tokens, see https://is.gd/WIXECN for documentation ")
	if err := prompt.SurveyAskOne(&survey.Password{
		Message: "Please enter the GitLab access token: ",
	}, &gl.personalAccessToken, survey.WithValidator(survey.Required)); err != nil {
		return err
	}

	return nil
}

func (gl *gitLabConfig) create() error {
	glClient, err := gl.newClient()
	if err != nil {
		return err
	}

	hookOpts := &gitlab.AddProjectHookOptions{
		EnableSSLVerification: gitlab.Bool(true),
		MergeRequestsEvents:   gitlab.Bool(true),
		NoteEvents:            gitlab.Bool(true),
		PushEvents:            gitlab.Bool(true),
		Token:                 gitlab.String(gl.webhookSecret),
		URL:                   gitlab.String(gl.controllerURL),
	}

	_, resp, err := glClient.Projects.AddProjectHook(gl.projectID, hookOpts)
	if err != nil {
		return err
	}

	if resp.Response.StatusCode != http.StatusCreated {
		payload, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}
		return fmt.Errorf("failed to create webhook, status code: %v, error : %v",
			resp.Response.StatusCode, payload)
	}

	// nolint:forbidigo
	fmt.Printf("✓ Webhook has been created on your repository\n")
	return nil
}

func (gl *gitLabConfig) newClient() (*gitlab.Client, error) {
	if gl.Client != nil {
		return gl.Client, nil
	}

	return gitlab.NewClient(gl.personalAccessToken, gitlab.WithBaseURL(gl.APIURL))
}
