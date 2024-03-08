package github

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	ghlib "github.com/google/go-github/v56/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/github"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/cctx"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/repository"
	"gotest.tools/v3/assert"
)

func Setup(ctx context.Context, onSecondController, viaDirectWebhook bool) (context.Context, *params.Run, options.E2E, *github.Provider, error) {
	githubToken := ""
	githubURL := os.Getenv("TEST_GITHUB_API_URL")
	githubRepoOwnerGithubApp := os.Getenv("TEST_GITHUB_REPO_OWNER_GITHUBAPP")
	githubRepoOwnerDirectWebhook := os.Getenv("TEST_GITHUB_REPO_OWNER_WEBHOOK")

	for _, value := range []string{
		"EL_URL",
		"GITHUB_API_URL",
		"GITHUB_TOKEN",
		"GITHUB_REPO_OWNER_GITHUBAPP",
		"EL_WEBHOOK_SECRET",
	} {
		if env := os.Getenv("TEST_" + value); env == "" {
			return ctx, nil, options.E2E{}, github.New(), fmt.Errorf("\"TEST_%s\" env variable is required, cannot continue", value)
		}
	}
	if onSecondController {
		for _, value := range []string{
			"TEST_GITHUB_SECOND_API_URL",
			"TEST_GITHUB_SECOND_REPO_OWNER_GITHUBAPP",
			"TEST_GITHUB_SECOND_TOKEN",
		} {
			if env := os.Getenv(value); env == "" {
				return ctx, nil, options.E2E{}, github.New(), fmt.Errorf("\"%s\" env variable is required for testing a second controller, cannot continue", value)
			}
		}
	}

	var split []string
	if !viaDirectWebhook {
		if githubURL == "" || githubRepoOwnerGithubApp == "" {
			return ctx, nil, options.E2E{}, github.New(), fmt.Errorf("TEST_GITHUB_API_URL TEST_GITHUB_REPO_OWNER_GITHUBAPP need to be set")
		}
		split = strings.Split(githubRepoOwnerGithubApp, "/")
	}
	if viaDirectWebhook {
		githubToken = os.Getenv("TEST_GITHUB_TOKEN")
		if githubURL == "" || githubToken == "" || githubRepoOwnerDirectWebhook == "" {
			return ctx, nil, options.E2E{}, github.New(), fmt.Errorf("TEST_GITHUB_API_URL TEST_GITHUB_TOKEN TEST_GITHUB_REPO_OWNER_WEBHOOK need to be set")
		}
		split = strings.Split(githubRepoOwnerDirectWebhook, "/")
	}
	if onSecondController {
		githubURL = os.Getenv("TEST_GITHUB_SECOND_API_URL")
		githubRepoOwnerGithubApp = os.Getenv("TEST_GITHUB_SECOND_REPO_OWNER_GITHUBAPP")
		githubToken = os.Getenv("TEST_GITHUB_SECOND_TOKEN")
		split = strings.Split(githubRepoOwnerGithubApp, "/")
	}

	run := params.New()
	if err := run.Clients.NewClients(ctx, &run.Info); err != nil {
		return ctx, nil, options.E2E{}, github.New(), err
	}
	run.Info.Controller = info.GetControllerInfoFromEnvOrDefault()

	controllerURL := os.Getenv("TEST_EL_URL")
	if controllerURL == "" {
		return ctx, nil, options.E2E{}, github.New(), fmt.Errorf("TEST_EL_URL variable is required, cannot continue")
	}

	e2eoptions := options.E2E{Organization: split[0], Repo: split[1], DirectWebhook: viaDirectWebhook, ControllerURL: controllerURL}
	gprovider := github.New()
	gprovider.Run = run
	event := info.NewEvent()

	if githubToken == "" && !viaDirectWebhook {
		var err error

		ctx, err = cctx.GetControllerCtxInfo(ctx, run)
		if err != nil {
			return ctx, nil, options.E2E{}, github.New(), err
		}

		envGithubRepoInstallationID := os.Getenv("TEST_GITHUB_REPO_INSTALLATION_ID")
		if envGithubRepoInstallationID == "" {
			return ctx, nil, options.E2E{}, github.New(), fmt.Errorf("TEST_GITHUB_REPO_INSTALLATION_ID need to be set")
		}
		// convert to int64 githubRepoInstallationID
		githubRepoInstallationID, err := strconv.ParseInt(envGithubRepoInstallationID, 10, 64)
		if err != nil {
			return ctx, nil, options.E2E{}, github.New(), fmt.Errorf("TEST_GITHUB_REPO_INSTALLATION_ID need to be set")
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
	// TODO: before PR
	if err := gprovider.SetClient(ctx, nil, event, nil, nil); err != nil {
		return ctx, nil, options.E2E{}, github.New(), err
	}

	return ctx, run, e2eoptions, gprovider, nil
}

func TearDown(ctx context.Context, t *testing.T, runcnx *params.Run, ghprovider *github.Provider, prNumber int, ref, targetNS string, opts options.E2E) {
	if os.Getenv("TEST_NOCLEANUP") == "true" {
		runcnx.Clients.Log.Infof("Not cleaning up and closing PR since TEST_NOCLEANUP is set")
		return
	}
	runcnx.Clients.Log.Infof("Closing PR %d", prNumber)
	if prNumber != -1 {
		state := "closed"
		_, _, err := ghprovider.Client.PullRequests.Edit(ctx,
			opts.Organization, opts.Repo, prNumber,
			&ghlib.PullRequest{State: &state})
		if err != nil {
			t.Fatal(err)
		}
	}
	repository.NSTearDown(ctx, t, runcnx, targetNS)
	runcnx.Clients.Log.Infof("Deleting Ref %s", ref)
	_, err := ghprovider.Client.Git.DeleteRef(ctx, opts.Organization, opts.Repo, ref)
	assert.NilError(t, err)
}
