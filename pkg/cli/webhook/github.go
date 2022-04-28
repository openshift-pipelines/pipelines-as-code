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
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"golang.org/x/oauth2"
)

const apiPublicURL = "https://api.github.com/"

type gitHubConfig struct {
	Client              *github.Client
	controllerURL       string
	repoOwner           string
	repoName            string
	webhookSecret       string
	personalAccessToken string
	APIURL              string
}

func (gh *gitHubConfig) GetName() string {
	return provider.ProviderGitHubWebhook
}

func (gh *gitHubConfig) Run(ctx context.Context, opts *Options) (*response, error) {
	err := gh.askGHWebhookConfig(opts.RepositoryURL, opts.ControllerURL)
	if err != nil {
		return nil, err
	}
	gh.APIURL = opts.ProviderAPIURL

	return &response{
		ControllerURL:       gh.controllerURL,
		PersonalAccessToken: gh.personalAccessToken,
		WebhookSecret:       gh.webhookSecret,
	}, gh.create(ctx)
}

func (gh *gitHubConfig) askGHWebhookConfig(repoURL, controllerURL string) error {
	var repo, defaultRepo string
	if repoURL != "" {
		if repo, _ := formatting.GetRepoOwnerFromGHURL(repoURL); repo != "" {
			defaultRepo = repo
		}
	}
	if defaultRepo != "" {
		msg := fmt.Sprintf("Please enter the repository you want to be configured (default: %s):", defaultRepo)
		if err := prompt.SurveyAskOne(&survey.Input{Message: msg}, &repo); err != nil {
			return err
		}
	} else {
		msg := "Please enter the repository you want to be configured (eg. repo-owner/repo-name) : "
		if err := prompt.SurveyAskOne(&survey.Input{Message: msg}, &repo,
			survey.WithValidator(survey.Required)); err != nil {
			return err
		}
	}

	if repo == "" {
		repo = defaultRepo
	}
	repoArr := strings.Split(repo, "/")
	if len(repoArr) != 2 {
		return fmt.Errorf("invalid repository, needs to be of format 'org-name/repo-name'")
	}

	gh.repoOwner = repoArr[0]
	gh.repoName = repoArr[1]

	// set controller url
	gh.controllerURL = controllerURL

	// confirm whether to use the detected url
	if gh.controllerURL != "" {
		var answer bool
		// nolint
		fmt.Printf("👀 I have detected a controller url: %s", gh.controllerURL)
		err := prompt.SurveyAskOne(&survey.Confirm{
			Message: "Do you want me to use it?",
			Default: true,
		}, &answer)
		if err != nil {
			return err
		}
		if !answer {
			gh.controllerURL = ""
		}
	}

	if gh.controllerURL == "" {
		if err := prompt.SurveyAskOne(&survey.Input{
			Message: "Please enter your controller public route URL: ",
		}, &gh.controllerURL, survey.WithValidator(survey.Required)); err != nil {
			return err
		}
	}

	if err := prompt.SurveyAskOne(&survey.Password{
		Message: "Please enter the secret to configure the webhook for payload validation: ",
	}, &gh.webhookSecret, survey.WithValidator(survey.Required)); err != nil {
		return err
	}

	// nolint:forbidigo
	fmt.Println("ℹ ️You now need to create a GitHub personal token with scopes  `public_repo` & `admin:repo_hook`")
	// nolint:forbidigo
	fmt.Println("ℹ ️Go to this URL to generate one https://github.com/settings/tokens/new, see https://is.gd/BMgLH5 for documentation ")
	if err := prompt.SurveyAskOne(&survey.Password{
		Message: "Please enter the GitHub access token: ",
	}, &gh.personalAccessToken, survey.WithValidator(survey.Required)); err != nil {
		return err
	}

	return nil
}

func (gh *gitHubConfig) create(ctx context.Context) error {
	hook := &github.Hook{
		Name:   github.String("web"),
		Active: github.Bool(true),
		Events: []string{
			"issue_comment",
			"pull_request",
			"push",
		},
		Config: map[string]interface{}{
			"url":          gh.controllerURL,
			"content_type": "json",
			"insecure_ssl": "0",
			"secret":       gh.webhookSecret,
		},
	}

	ghClient, err := gh.newGHClientByToken(ctx)
	if err != nil {
		return err
	}

	_, res, err := ghClient.Repositories.CreateHook(ctx, gh.repoOwner, gh.repoName, hook)
	if err != nil {
		return err
	}

	if res.Response.StatusCode != http.StatusCreated {
		payload, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}

		return fmt.Errorf("failed to create webhook on repository %v/%v, status code: %v, error : %v",
			gh.repoOwner, gh.repoName, res.Response.StatusCode, payload)
	}

	// nolint:forbidigo
	fmt.Printf("✓ Webhook has been created on repository %v/%v\n", gh.repoOwner, gh.repoName)
	return nil
}

func (gh *gitHubConfig) newGHClientByToken(ctx context.Context) (*github.Client, error) {
	if gh.Client != nil {
		return gh.Client, nil
	}
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: gh.personalAccessToken},
	)

	if gh.APIURL == "" || gh.APIURL == apiPublicURL {
		return github.NewClient(oauth2.NewClient(ctx, ts)), nil
	}

	gprovider, err := github.NewEnterpriseClient(gh.APIURL, "", oauth2.NewClient(ctx, ts))
	if err != nil {
		return nil, err
	}
	return gprovider, nil
}
