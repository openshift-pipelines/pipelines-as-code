package github

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/github"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/cctx"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/setup"
)

func Setup(ctx context.Context, onGHE, viaDirectWebhook bool) (context.Context, *params.Run, options.E2E, *github.Provider, error) {
	if err := setup.RequireEnvs(
		"TEST_EL_URL",
		"TEST_GITHUB_API_URL",
		"TEST_GITHUB_TOKEN",
		"TEST_GITHUB_REPO_OWNER_GITHUBAPP",
		"TEST_EL_WEBHOOK_SECRET",
	); err != nil {
		return ctx, nil, options.E2E{}, github.New(), err
	}

	githubToken := ""
	githubURL := os.Getenv("TEST_GITHUB_API_URL")
	githubRepoOwnerGithubApp := os.Getenv("TEST_GITHUB_REPO_OWNER_GITHUBAPP")
	// EL_URL mean CONTROLLER URL, it's called el_url because a long time ago pac was based on trigger
	controllerURL := os.Getenv("TEST_EL_URL")

	if onGHE {
		requiredEnvs := []string{
			"TEST_GITHUB_SECOND_API_URL",
			"TEST_GITHUB_SECOND_REPO_OWNER_GITHUBAPP",
			"TEST_GITHUB_SECOND_TOKEN",
			"TEST_GITHUB_SECOND_EL_URL",
		}
		if viaDirectWebhook {
			requiredEnvs = append(requiredEnvs, "TEST_GITHUB_SECOND_WEBHOOK_SMEE_URL")
		}
		if err := setup.RequireEnvs(requiredEnvs...); err != nil {
			return ctx, nil, options.E2E{}, github.New(), err
		}
	}

	var split []string
	var repo string

	// Configure based on provider and authentication method
	switch {
	case onGHE && viaDirectWebhook:
		// GHE + webhook (dynamic repo creation)
		githubURL = os.Getenv("TEST_GITHUB_SECOND_API_URL")
		githubToken = os.Getenv("TEST_GITHUB_SECOND_TOKEN")
		controllerURL = os.Getenv("TEST_GITHUB_SECOND_EL_URL")
		// Use dedicated webhook org if set, otherwise fall back to GitHub App org
		webhookOrg := os.Getenv("TEST_GITHUB_SECOND_WEBHOOK_ORG")
		if webhookOrg != "" {
			split = []string{webhookOrg}
		} else {
			githubRepoOwnerGithubApp = os.Getenv("TEST_GITHUB_SECOND_REPO_OWNER_GITHUBAPP")
			split = strings.Split(githubRepoOwnerGithubApp, "/")
		}
		repo = "" // Will be filled after dynamic repo creation
	case onGHE:
		// GHE + GitHub App
		githubURL = os.Getenv("TEST_GITHUB_SECOND_API_URL")
		githubRepoOwnerGithubApp = os.Getenv("TEST_GITHUB_SECOND_REPO_OWNER_GITHUBAPP")
		githubToken = os.Getenv("TEST_GITHUB_SECOND_TOKEN")
		controllerURL = os.Getenv("TEST_GITHUB_SECOND_EL_URL")
		split = strings.Split(githubRepoOwnerGithubApp, "/")
		repo = split[1]
	case viaDirectWebhook:
		// Public GitHub + webhook (uses same repo as GitHub App)
		githubToken = os.Getenv("TEST_GITHUB_TOKEN")
		split = strings.Split(githubRepoOwnerGithubApp, "/")
		repo = split[1]
	default:
		// Public GitHub + GitHub App
		split = strings.Split(githubRepoOwnerGithubApp, "/")
		repo = split[1]
	}

	run := params.New()
	if err := run.Clients.NewClients(ctx, &run.Info); err != nil {
		return ctx, nil, options.E2E{}, github.New(), err
	}
	run.Info.Controller = info.GetControllerInfoFromEnvOrDefault()
	ctxWithInfo, err := cctx.GetControllerCtxInfo(ctx, run)
	if err != nil {
		return ctx, nil, options.E2E{}, github.New(), err
	}
	ctx = ctxWithInfo
	e2eoptions := options.E2E{
		Organization:  split[0],
		Repo:          repo,
		DirectWebhook: viaDirectWebhook,
		ControllerURL: controllerURL,
		Token:         githubToken,
		APIURL:        githubURL,
	}
	gprovider := github.New()
	gprovider.Run = run
	event := info.NewEvent()

	if githubToken == "" && !viaDirectWebhook {
		var err error

		envGithubRepoInstallationID, err := setup.GetRequiredEnv("TEST_GITHUB_REPO_INSTALLATION_ID")
		if err != nil {
			return ctx, nil, options.E2E{}, github.New(), err
		}
		// convert to int64 githubRepoInstallationID
		githubRepoInstallationID, err := strconv.ParseInt(envGithubRepoInstallationID, 10, 64)
		if err != nil {
			return ctx, nil, options.E2E{}, github.New(), fmt.Errorf("TEST_GITHUB_REPO_INSTALLATION_ID env variable must be an integer but got '%s'", envGithubRepoInstallationID)
		}
		ns := info.GetNS(ctx)
		githubToken, err = gprovider.GetAppToken(ctx, run.Clients.Kube, githubURL, githubRepoInstallationID, ns)
		if err != nil {
			return ctx, nil, options.E2E{}, github.New(), err
		}
	}

	event.Provider = &info.Provider{
		Token: githubToken,
		URL:   githubURL,
	}
	gprovider.Token = &githubToken
	// TODO: before PR
	if err := gprovider.SetClient(ctx, run, event, nil, nil); err != nil {
		return ctx, nil, options.E2E{}, github.New(), err
	}

	return ctx, run, e2eoptions, gprovider, nil
}
