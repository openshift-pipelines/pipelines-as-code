package bitbucketcloud

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/ktrysmt/go-bitbucket"
	"gotest.tools/v3/assert"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/bitbucketcloud"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/repository"
)

func Setup(ctx context.Context) (*params.Run, options.E2E, bitbucketcloud.Provider, error) {
	bitbucketCloudUser := os.Getenv("TEST_BITBUCKET_CLOUD_USER")
	bitbucketCloudToken := os.Getenv("TEST_BITBUCKET_CLOUD_TOKEN")
	bitbucketWSOwner := os.Getenv("TEST_BITBUCKET_CLOUD_E2E_REPOSITORY")
	bitbucketCloudAPIURL := os.Getenv("TEST_BITBUCKET_CLOUD_API_URL")

	for _, value := range []string{
		"BITBUCKET_CLOUD_TOKEN", "BITBUCKET_CLOUD_E2E_REPOSITORY", "BITBUCKET_CLOUD_API_URL",
	} {
		if env := os.Getenv("TEST_" + value); env == "" {
			return nil, options.E2E{}, bitbucketcloud.Provider{}, fmt.Errorf("\"TEST_%s\" env variable is required, skipping", value)
		}
	}

	split := strings.Split(bitbucketWSOwner, "/")

	run := &params.Run{}
	if err := run.Clients.NewClients(ctx, &run.Info); err != nil {
		return nil, options.E2E{}, bitbucketcloud.Provider{}, err
	}
	e2eoptions := options.E2E{
		Organization: split[0],
		Repo:         split[1],
	}
	bbc := bitbucketcloud.Provider{}
	event := info.NewEvent()
	event.Provider = &info.Provider{
		Token: bitbucketCloudToken,
		URL:   bitbucketCloudAPIURL,
		User:  bitbucketCloudUser,
	}
	if err := bbc.SetClient(ctx, nil, event, nil); err != nil {
		return nil, options.E2E{}, bitbucketcloud.Provider{}, err
	}
	return run, e2eoptions, bbc, nil
}

func TearDown(ctx context.Context, t *testing.T, runcnx *params.Run, bprovider bitbucketcloud.Provider, opts options.E2E, prNumber int, ref, targetNS string) {
	if os.Getenv("TEST_NOCLEANUP") == "true" {
		runcnx.Clients.Log.Infof("Not cleaning up and closing PR since TEST_NOCLEANUP is set")
		return
	}
	runcnx.Clients.Log.Infof("Closing PR #%d", prNumber)
	_, err := bprovider.Client.Repositories.PullRequests.Decline(&bitbucket.PullRequestsOptions{
		ID:       fmt.Sprintf("%d", prNumber),
		Owner:    opts.Organization,
		RepoSlug: opts.Repo,
	})
	assert.NilError(t, err)
	runcnx.Clients.Log.Infof("Deleting ref %s", ref)
	err = bprovider.Client.Repositories.Repository.DeleteBranch(
		&bitbucket.RepositoryBranchDeleteOptions{
			Owner:    opts.Organization,
			RepoSlug: opts.Repo,
			RefName:  ref,
		},
	)
	assert.NilError(t, err)

	repository.NSTearDown(ctx, t, runcnx, targetNS)
}
