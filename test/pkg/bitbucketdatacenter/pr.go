package bitbucketdatacenter

import (
	"context"
	"fmt"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/payload"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/scm"

	goscm "github.com/jenkins-x/go-scm/scm"
	"gotest.tools/v3/assert"
)

func CreatePR(ctx context.Context, t *testing.T, client *goscm.Client, runcnx *params.Run, opts options.E2E, repo *goscm.Repository, files map[string]string, orgAndRepo, targetNS string) *goscm.PullRequest {
	mainBranchRef := "refs/heads/main"
	branch, resp, err := client.Git.CreateRef(ctx, orgAndRepo, targetNS, mainBranchRef)
	assert.NilError(t, err, "error creating branch: http status code: %d : %v", resp.Status, err)
	runcnx.Clients.Log.Infof("Branch %s has been created", branch.Name)

	files, err = payload.GetEntries(files, targetNS, options.MainBranch, triggertype.PullRequest.String(), map[string]string{})
	assert.NilError(t, err)
	gitCloneURL, err := scm.MakeGitCloneURL(repo.Clone, opts.UserName, opts.Password)
	assert.NilError(t, err)
	scmOpts := &scm.Opts{
		GitURL:        gitCloneURL,
		Log:           runcnx.Clients.Log,
		WebURL:        repo.Clone,
		TargetRefName: targetNS,
		BaseRefName:   repo.Branch,
		CommitTitle:   fmt.Sprintf("commit %s", targetNS),
	}
	scm.PushFilesToRefGit(t, scmOpts, files)
	runcnx.Clients.Log.Infof("Branch %s has been created and pushed with files", targetNS)

	title := "TestPullRequest - " + targetNS
	prOpts := &goscm.PullRequestInput{
		Title: title,
		Body:  "Test PAC on bitbucket data center",
		Head:  targetNS,
		Base:  "main",
	}
	pr, resp, err := client.PullRequests.Create(ctx, orgAndRepo, prOpts)
	assert.NilError(t, err, "error creating pull request: http status code: %d : %v", resp.Status, err)
	return pr
}
