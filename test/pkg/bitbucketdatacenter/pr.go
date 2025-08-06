package bitbucketdatacenter

import (
	"context"
	"fmt"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/scm"

	goscm "github.com/jenkins-x/go-scm/scm"
	"gotest.tools/v3/assert"
)

func CreatePR(ctx context.Context, t *testing.T, client *goscm.Client, runcnx *params.Run, opts options.E2E, repo *goscm.Repository, files map[string]string, orgAndRepo, targetNS string) *goscm.PullRequest {
	baseBranchRef := repo.Branch
	if opts.BaseBranch != "" {
		baseBranchRef = opts.BaseBranch
	}

	branch, resp, err := client.Git.CreateRef(ctx, orgAndRepo, targetNS, baseBranchRef)
	msg := "error creating branch"
	if resp != nil {
		msg = fmt.Sprintf("error creating branch: http status code: %d : %v", resp.Status, err)
	} else if err != nil {
		msg = fmt.Sprintf("error creating branch: %v", err)
	}
	assert.NilError(t, err, msg)
	runcnx.Clients.Log.Infof("Branch %s has been created", branch.Name)

	gitCloneURL, err := scm.MakeGitCloneURL(repo.Clone, opts.UserName, opts.Password)
	assert.NilError(t, err)

	scmOpts := &scm.Opts{
		GitURL:        gitCloneURL,
		Log:           runcnx.Clients.Log,
		WebURL:        repo.Clone,
		TargetRefName: targetNS,
		BaseRefName:   baseBranchRef,
		CommitTitle:   fmt.Sprintf("commit %s", targetNS),
	}
	scm.PushFilesToRefGit(t, scmOpts, files)
	runcnx.Clients.Log.Infof("Branch %s has been created and pushed with files", targetNS)

	title := "TestPullRequest - " + targetNS
	prOpts := &goscm.PullRequestInput{
		Title: title,
		Body:  "Test PAC on bitbucket data center",
		Head:  targetNS,
		Base:  baseBranchRef,
	}
	pr, resp, err := client.PullRequests.Create(ctx, orgAndRepo, prOpts)
	msg = "error creating pull request"
	if resp != nil {
		msg = fmt.Sprintf("error creating pull request: http status code: %d : %v", resp.Status, err)
	} else if err != nil {
		msg = fmt.Sprintf("error creating pull request: %v", err)
	}
	assert.NilError(t, err, msg)
	runcnx.Clients.Log.Infof("Created Pull Request with Title '%s'. Head branch '%s' ⮕ Base Branch '%s'", pr.Title, pr.Head.Ref, pr.Base.Ref)
	return pr
}
