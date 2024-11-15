package bitbucketserver

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/bitbucketserver"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/repository"

	"github.com/antihax/optional"
	bbrest "github.com/gdasson/bitbucketv1go"
	"gotest.tools/v3/assert"
)

func Setup(ctx context.Context) (context.Context, *params.Run, options.E2E, bitbucketserver.Provider, *bbrest.APIClient, error) {
	bitbucketServerUser := os.Getenv("TEST_BITBUCKET_SERVER_USER")
	bitbucketServerToken := os.Getenv("TEST_BITBUCKET_SERVER_TOKEN")
	bitbucketWSOwner := os.Getenv("TEST_BITBUCKET_SERVER_E2E_REPOSITORY")
	bitbucketServerAPIURL := os.Getenv("TEST_BITBUCKET_SERVER_API_URL")

	for _, value := range []string{
		"BITBUCKET_SERVER_USER", "BITBUCKET_SERVER_TOKEN", "BITBUCKET_SERVER_E2E_REPOSITORY", "BITBUCKET_SERVER_API_URL", "BITBUCKET_SERVER_WEBHOOK_SECRET",
	} {
		if env := os.Getenv("TEST_" + value); env == "" {
			return ctx, nil, options.E2E{}, bitbucketserver.Provider{}, nil, fmt.Errorf("\"TEST_%s\" env variable is required, skipping", value)
		}
	}

	split := strings.Split(bitbucketWSOwner, "/")

	run := params.New()
	if err := run.Clients.NewClients(ctx, &run.Info); err != nil {
		return ctx, nil, options.E2E{}, bitbucketserver.Provider{}, nil, err
	}
	e2eoptions := options.E2E{
		Organization: split[0],
		Repo:         split[1],
	}
	bbs := bitbucketserver.Provider{}
	event := info.NewEvent()
	event.Provider = &info.Provider{
		Token: bitbucketServerToken,
		URL:   bitbucketServerAPIURL,
		User:  bitbucketServerUser,
	}
	if err := bbs.SetClient(ctx, nil, event, nil, nil); err != nil {
		return ctx, nil, options.E2E{}, bitbucketserver.Provider{}, nil, err
	}

	cfg := bbrest.NewConfiguration()
	cfg.BasePath = bitbucketServerAPIURL
	restClient := bbrest.NewAPIClient(cfg)
	basicAuth := bbrest.BasicAuth{UserName: bitbucketServerUser, Password: bitbucketServerToken}
	ctx = context.WithValue(ctx, bbrest.ContextBasicAuth, basicAuth)
	return ctx, run, e2eoptions, bbs, restClient, nil
}

func TearDownNs(ctx context.Context, t *testing.T, runcnx *params.Run, targetNS string) {
	if os.Getenv("TEST_NOCLEANUP") == "true" {
		runcnx.Clients.Log.Infof("Not cleaning up and closing PR since TEST_NOCLEANUP is set")
		return
	}

	repository.NSTearDown(ctx, t, runcnx, targetNS)
}

func TearDownBranch(ctx context.Context, t *testing.T, runcnx *params.Run, bprovider bitbucketserver.Provider, restClient *bbrest.APIClient, opts options.E2E, prID int64, ref string) {
	if os.Getenv("TEST_NOCLEANUP") == "true" {
		runcnx.Clients.Log.Infof("Not cleaning up and closing PR since TEST_NOCLEANUP is set")
		return
	}

	if prID != -1 {
		runcnx.Clients.Log.Infof("Deleting PR #%d", prID)
		_, err := bprovider.Client.DefaultApi.DeletePullRequest(opts.Organization, opts.Repo, int(prID))
		assert.NilError(t, err)
	}

	if ref != "" {
		runcnx.Clients.Log.Infof("Deleting Branch %s", ref)
		request := &bbrest.RepositoryApiDeleteBranchOpts{Body: optional.NewInterface(map[string]string{"name": ref})}
		resp, err := restClient.RepositoryApi.DeleteBranch(ctx, opts.Organization, opts.Repo, request)
		assert.NilError(t, err)
		defer resp.Body.Close()
	}
}
