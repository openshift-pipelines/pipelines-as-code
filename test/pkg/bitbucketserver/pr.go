package bitbucketserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/payload"

	"github.com/antihax/optional"
	bbrest "github.com/gdasson/bitbucketv1go"
	bbv1 "github.com/gfleury/go-bitbucket-v1"
	"gotest.tools/v3/assert"
)

func CreatePR(ctx context.Context, t *testing.T, restClient *bbrest.APIClient, repo bbv1.Repository, runcnx *params.Run, opts options.E2E, files map[string]string, title, targetNS string) (bbrest.RestPullRequest, []*bbrest.RestCommit) {
	mainBranchRef := "refs/heads/main"
	branchCreateRequest := bbrest.RestBranchCreateRequest{Name: targetNS, StartPoint: mainBranchRef}
	branch, resp, err := restClient.RepositoryApi.CreateBranch(ctx, branchCreateRequest, opts.Organization, opts.Repo)
	assert.NilError(t, err)
	defer resp.Body.Close()
	runcnx.Clients.Log.Infof("Branch %s is created", branch.Id)

	files, err = payload.GetEntries(files, targetNS, options.MainBranch, triggertype.PullRequest.String(), map[string]string{})
	assert.NilError(t, err)
	commits, err := pushFilesToBranch(ctx, runcnx, opts, targetNS, files)
	assert.NilError(t, err)

	prCreateOpts := &bbrest.PullRequestsApiCreateOpts{Body: optional.NewInterface(map[string]interface{}{
		"title":       title,
		"description": "Test PAC on bitbucket server",
		"fromRef":     bbv1.PullRequestRef{ID: branch.Id, Repository: repo},
		"toRef":       bbv1.PullRequestRef{ID: mainBranchRef, Repository: repo},
		"closed":      false,
	})}
	pr, resp, err := restClient.PullRequestsApi.Create(ctx, opts.Organization, opts.Repo, prCreateOpts)
	assert.NilError(t, err)
	defer resp.Body.Close()

	return pr, commits
}

// pushFilesToBranch pushes multiple files to bitbucket server repo because
// bitbucket server doesn't support uploading multiple files in an API call.
// reference: https://community.developer.atlassian.com/t/rest-api-to-update-multiple-files/28731/2
// and new rest client library is also having some issue with EditFile function
// when tried with same creds, form fields and file as used in pushFilesToBranch function.
func pushFilesToBranch(ctx context.Context, run *params.Run, opts options.E2E, branchName string, files map[string]string) ([]*bbrest.RestCommit, error) {
	if len(files) == 0 {
		return nil, fmt.Errorf("no file to commit")
	}

	commits := make([]*bbrest.RestCommit, 0, len(files))
	apiURL := os.Getenv("TEST_BITBUCKET_SERVER_API_URL")
	for file, content := range files {
		endpointURL := fmt.Sprintf("%s/api/latest/projects/%s/repos/%s/browse/.tekton/%s", apiURL, opts.Organization, opts.Repo, file)
		fields := map[string]string{
			"branch":  branchName,
			"content": content,
			"message": "test commit Signed-off-by: Zaki Shaikh <zashaikh@redhat.com>",
		}

		response, err := callAPI(ctx, endpointURL, http.MethodPut, fields)
		if err != nil {
			return nil, err
		}

		var tmpCommit bbrest.RestCommit
		err = json.Unmarshal(response, &tmpCommit)
		if err != nil {
			return nil, fmt.Errorf("error unmarshaling response: %w", err)
		}
		commits = append(commits, &tmpCommit)
	}
	run.Clients.Log.Infof("%d files committed to branch %s", len(files), branchName)

	return commits, nil
}
