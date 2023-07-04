package gitea

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/google/go-github/v53/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/gitea"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/repository"
	"gotest.tools/v3/assert"
)

func CreateProvider(ctx context.Context, giteaURL, user, password string) (gitea.Provider, error) {
	run := &params.Run{}
	if err := run.Clients.NewClients(ctx, &run.Info); err != nil {
		return gitea.Provider{}, fmt.Errorf("cannot create new client: %w", err)
	}
	gprovider := gitea.Provider{
		Password: password,
		Token:    github.String(password),
	}
	event := info.NewEvent()
	event.Provider = &info.Provider{
		URL:   giteaURL,
		User:  user,
		Token: password,
	}
	if err := gprovider.SetClient(ctx, nil, event, nil); err != nil {
		return gitea.Provider{}, fmt.Errorf("cannot set client: %w", err)
	}
	return gprovider, nil
}

func Setup(ctx context.Context) (*params.Run, options.E2E, gitea.Provider, error) {
	giteaURL := os.Getenv("TEST_GITEA_API_URL")
	giteaPassword := os.Getenv("TEST_GITEA_PASSWORD")
	giteaRepoOwner := os.Getenv("TEST_GITEA_REPO_OWNER")

	for _, value := range []string{
		"EL_URL",
		"GITEA_API_URL",
		"GITEA_PASSWORD",
		"GITEA_REPO_OWNER",
		"EL_WEBHOOK_SECRET",
		"GITEA_SMEEURL",
	} {
		if env := os.Getenv("TEST_" + value); env == "" {
			return nil, options.E2E{}, gitea.Provider{}, fmt.Errorf("\"TEST_%s\" env variable is required, cannot continue", value)
		}
	}

	var splitted []string
	if giteaURL == "" || giteaPassword == "" || giteaRepoOwner == "" {
		return nil, options.E2E{}, gitea.Provider{}, fmt.Errorf("TEST_GITEA_API_URL TEST_GITEA_PASSWORD TEST_GITEA_REPO_OWNER need to be set")
	}
	splitted = strings.Split(giteaRepoOwner, "/")

	run := &params.Run{}
	if err := run.Clients.NewClients(ctx, &run.Info); err != nil {
		return nil, options.E2E{}, gitea.Provider{}, fmt.Errorf("cannot create new client: %w", err)
	}
	// Repo is actually not used
	e2eoptions := options.E2E{Organization: splitted[0], Repo: splitted[1]}
	gprovider, err := CreateProvider(ctx, giteaURL, splitted[0], giteaPassword)
	if err != nil {
		return nil, options.E2E{}, gitea.Provider{}, fmt.Errorf("cannot set client: %w", err)
	}

	return run, e2eoptions, gprovider, nil
}

func TearDown(ctx context.Context, t *testing.T, topts *TestOpts) {
	repository.NSTearDown(ctx, t, topts.ParamsRun, topts.TargetNS)
	_, err := topts.GiteaCNX.Client.DeleteRepo(topts.Opts.Organization, topts.TargetRefName)
	assert.NilError(t, err)
}
