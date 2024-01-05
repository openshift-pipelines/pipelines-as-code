package gitea

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/google/go-github/v56/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/gitea"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/repository"
)

func CreateProvider(ctx context.Context, giteaURL, user, password string) (gitea.Provider, error) {
	run := params.New()
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
	if err := gprovider.SetClient(ctx, nil, event, nil, nil); err != nil {
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

	var split []string
	if giteaURL == "" || giteaPassword == "" || giteaRepoOwner == "" {
		return nil, options.E2E{}, gitea.Provider{}, fmt.Errorf("TEST_GITEA_API_URL TEST_GITEA_PASSWORD TEST_GITEA_REPO_OWNER need to be set")
	}
	split = strings.Split(giteaRepoOwner, "/")

	run := params.New()
	if err := run.Clients.NewClients(ctx, &run.Info); err != nil {
		return nil, options.E2E{}, gitea.Provider{}, fmt.Errorf("cannot create new client: %w", err)
	}
	// Repo is actually not used
	e2eoptions := options.E2E{Organization: split[0], Repo: split[1]}
	gprovider, err := CreateProvider(ctx, giteaURL, split[0], giteaPassword)
	if err != nil {
		return nil, options.E2E{}, gitea.Provider{}, fmt.Errorf("cannot set client: %w", err)
	}

	return run, e2eoptions, gprovider, nil
}

func TearDown(ctx context.Context, t *testing.T, topts *TestOpts) {
	if os.Getenv("TEST_NOCLEANUP") == "true" {
		topts.ParamsRun.Clients.Log.Infof("Not cleaning up and closing PR since TEST_NOCLEANUP is set")
		return
	}
	repository.NSTearDown(ctx, t, topts.ParamsRun, topts.TargetNS)
	_, err := topts.GiteaCNX.Client.DeleteRepo(topts.Opts.Organization, topts.TargetNS)
	if err != nil {
		t.Logf("Error deleting repo %s/%s: %s", topts.Opts.Organization, topts.TargetNS, err)
	} else {
		t.Logf("Deleted repo %s/%s", topts.Opts.Organization, topts.TargetNS)
	}
}
