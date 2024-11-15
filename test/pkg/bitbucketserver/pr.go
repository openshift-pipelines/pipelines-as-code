package bitbucketserver

import (
	"context"
	"fmt"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/payload"

	"github.com/jenkins-x/go-scm/scm"
	"gotest.tools/v3/assert"
)

func CreatePR(ctx context.Context, t *testing.T, client *scm.Client, runcnx *params.Run, orgAndRepo string, files map[string]string, title, targetNS string) *scm.PullRequest {
	mainBranchRef := "refs/heads/main"
	branch, resp, err := client.Git.CreateRef(ctx, orgAndRepo, targetNS, mainBranchRef)
	assert.NilError(t, err, "error creating branch: http status code: %d : %v", resp.Status, err)
	runcnx.Clients.Log.Infof("Branch %s has been created", branch.Name)

	files, err = payload.GetEntries(files, targetNS, options.MainBranch, triggertype.PullRequest.String(), map[string]string{})
	assert.NilError(t, err)
	err = pushFilesToBranch(ctx, runcnx, client, orgAndRepo, targetNS, files)
	assert.NilError(t, err, "error pushing files to branch: %v", err)

	prOpts := &scm.PullRequestInput{
		Title: title,
		Body:  "Test PAC on bitbucket server",
		Head:  targetNS,
		Base:  "main",
	}
	pr, resp, err := client.PullRequests.Create(ctx, orgAndRepo, prOpts)
	assert.NilError(t, err, "error creating pull request: http status code: %d : %v", resp.Status, err)
	return pr
}

// pushFilesToBranch pushes multiple files to bitbucket server repo because
// bitbucket server doesn't support uploading multiple files in an API call.
// reference: https://community.developer.atlassian.com/t/rest-api-to-update-multiple-files/28731/2
func pushFilesToBranch(ctx context.Context, run *params.Run, client *scm.Client, repoAndOrg, branchName string, files map[string]string) error {
	if len(files) == 0 {
		return fmt.Errorf("no file to commit")
	}

	for file, content := range files {
		param := &scm.ContentParams{
			Branch:    branchName,
			Message:   "test commit",
			Data:      []byte(content),
			Signature: scm.Signature{Name: "Zaki Shaikh", Email: "zaki@example.com"},
		}
		path := fmt.Sprintf(".tekton/%s", file)
		_, err := client.Contents.Create(ctx, repoAndOrg, path, param)
		if err != nil {
			return err
		}
	}
	run.Clients.Log.Infof("%d files committed to branch %s", len(files), branchName)

	return nil
}
