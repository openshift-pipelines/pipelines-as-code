package github

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	ghlib "github.com/google/go-github/v42/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/github"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/repository"
	"gotest.tools/v3/assert"
)

func Setup(ctx context.Context, viaDirectWebhook bool) (*params.Run, options.E2E, github.Provider, error) {
	githubURL := os.Getenv("TEST_GITHUB_API_URL")
	githubToken := os.Getenv("TEST_GITHUB_TOKEN")
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
			return nil, options.E2E{}, github.Provider{}, fmt.Errorf("\"TEST_%s\" env variable is required, cannot continue", value)
		}
	}

	var splitted []string
	if !viaDirectWebhook {
		if githubURL == "" || githubToken == "" || githubRepoOwnerGithubApp == "" {
			return nil, options.E2E{}, github.Provider{}, fmt.Errorf("TEST_GITHUB_API_URL TEST_GITHUB_TOKEN TEST_GITHUB_REPO_OWNER_GITHUBAPP need to be set")
		}
		splitted = strings.Split(githubRepoOwnerGithubApp, "/")
	}
	if viaDirectWebhook {
		if githubURL == "" || githubToken == "" || githubRepoOwnerDirectWebhook == "" {
			return nil, options.E2E{}, github.Provider{}, fmt.Errorf("TEST_GITHUB_API_URL TEST_GITHUB_TOKEN TEST_GITHUB_REPO_OWNER_WEBHOOK need to be set")
		}
		splitted = strings.Split(githubRepoOwnerDirectWebhook, "/")
	}

	run := &params.Run{}
	if err := run.Clients.NewClients(ctx, &run.Info); err != nil {
		return nil, options.E2E{}, github.Provider{}, err
	}
	e2eoptions := options.E2E{Organization: splitted[0], Repo: splitted[1], DirectWebhook: viaDirectWebhook}
	gprovider := github.Provider{}
	if err := gprovider.SetClient(ctx, &info.PacOpts{ProviderToken: githubToken, ProviderURL: githubURL}); err != nil {
		return nil, options.E2E{}, github.Provider{}, err
	}

	return run, e2eoptions, gprovider, nil
}

func TearDown(ctx context.Context, t *testing.T, runcnx *params.Run, ghprovider github.Provider, prNumber int, ref string, targetNS string, opts options.E2E) {
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
