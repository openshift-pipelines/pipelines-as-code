package webhook

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/google/go-github/v43/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli/prompt"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/formatting"
	"golang.org/x/oauth2"
)

const apiPublicURL = "https://api.github.com/"

type gitHubWebhookConfig struct {
	ControllerURL       string
	RepoOwner           string
	RepoName            string
	WebhookSecret       string
	PersonalAccessToken string
	APIURL              string
}

func (w Webhook) githubWebhook(ctx context.Context) (*response, error) {
	msg := "Would you like me to configure a GitHub Webhook for your repository? "
	var configureWebhook bool
	if err := prompt.SurveyAskOne(&survey.Confirm{Message: msg, Default: true}, &configureWebhook); err != nil {
		return nil, err
	}
	if !configureWebhook {
		return &response{UserDeclined: true}, nil
	}

	ghWebhook, err := askGHWebhookConfig(w.RepositoryURL, w.ControllerURL)
	if err != nil {
		return nil, err
	}
	ghWebhook.APIURL = w.ProviderAPIURL

	return &response{
		UserDeclined:        false,
		ControllerURL:       ghWebhook.ControllerURL,
		PersonalAccessToken: ghWebhook.PersonalAccessToken,
		WebhookSecret:       ghWebhook.WebhookSecret,
	}, ghWebhook.create(ctx)
}

func askGHWebhookConfig(repoURL, controllerURL string) (*gitHubWebhookConfig, error) {
	gh := &gitHubWebhookConfig{}

	var repo, defaultRepo string
	if repoURL != "" {
		if repo, _ := formatting.GetRepoOwnerFromGHURL(repoURL); repo != "" {
			defaultRepo = repo
		}
	}
	if defaultRepo != "" {
		msg := fmt.Sprintf("Please enter the repository you want to be configured (default: %s):", defaultRepo)
		if err := prompt.SurveyAskOne(&survey.Input{Message: msg}, &repo); err != nil {
			return nil, err
		}
	} else {
		msg := "Please enter the repository you want to be configured (eg. repo-owner/repo-name) : "
		if err := prompt.SurveyAskOne(&survey.Input{Message: msg}, &repo,
			survey.WithValidator(survey.Required)); err != nil {
			return nil, err
		}
	}

	if repo == "" {
		repo = defaultRepo
	}
	repoArr := strings.Split(repo, "/")
	if len(repoArr) != 2 {
		return nil, fmt.Errorf("invalid repository, needs to be of format 'org-name/repo-name'")
	}

	gh.RepoOwner = repoArr[0]
	gh.RepoName = repoArr[1]

	if controllerURL == "" {
		if err := prompt.SurveyAskOne(&survey.Input{
			Message: "Please enter your controller public route URL: ",
		}, &gh.ControllerURL, survey.WithValidator(survey.Required)); err != nil {
			return nil, err
		}
	}

	if err := prompt.SurveyAskOne(&survey.Input{
		Message: "Please enter the secret to configure the webhook for payload validation: ",
	}, &gh.WebhookSecret, survey.WithValidator(survey.Required)); err != nil {
		return nil, err
	}

	// nolint:forbidigo
	fmt.Println("ℹ ️You now need to create a GitHub personal token with scopes  `public_repo` & `admin:repo_hook`")
	// nolint:forbidigo
	fmt.Println("ℹ ️Go to this URL to generate one https://github.com/settings/tokens/new, see https://is.gd/BMgLH5 for documentation ")
	if err := prompt.SurveyAskOne(&survey.Input{
		Message: "Please enter the GitHub access token: ",
	}, &gh.PersonalAccessToken, survey.WithValidator(survey.Required)); err != nil {
		return nil, err
	}

	return gh, nil
}

func (gh gitHubWebhookConfig) create(ctx context.Context) error {
	hook := &github.Hook{
		Name:   github.String("web"),
		Active: github.Bool(true),
		Events: []string{
			"issue_comment",
			"pull_request",
			"push",
		},
		Config: map[string]interface{}{
			"url":          gh.ControllerURL,
			"content_type": "json",
			"insecure_ssl": "0",
			"secret":       gh.WebhookSecret,
		},
	}

	ghClient, err := newGHClientByToken(ctx, gh.PersonalAccessToken, gh.APIURL)
	if err != nil {
		return err
	}

	_, res, err := ghClient.Repositories.CreateHook(ctx, gh.RepoOwner, gh.RepoName, hook)
	if err != nil {
		return err
	}

	if res.Response.StatusCode != http.StatusCreated {
		payload, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}

		return fmt.Errorf("failed to create webhook on repository %v/%v, status code: %v, error : %v",
			gh.RepoOwner, gh.RepoName, res.Response.StatusCode, payload)
	}

	// nolint:forbidigo
	fmt.Printf("✓ Webhook has been created on repository %v/%v\n", gh.RepoOwner, gh.RepoName)
	return nil
}

func newGHClientByToken(ctx context.Context, token, apiURL string) (*github.Client, error) {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)

	if apiURL == "" || apiURL == apiPublicURL {
		return github.NewClient(oauth2.NewClient(ctx, ts)), nil
	}

	gprovider, err := github.NewEnterpriseClient(apiURL, "", oauth2.NewClient(ctx, ts))
	if err != nil {
		return nil, err
	}
	return gprovider, nil
}
