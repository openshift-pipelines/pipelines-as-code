package bitbucketcloud

import (
	"fmt"
	"testing"

	"github.com/ktrysmt/go-bitbucket"
	"github.com/mitchellh/mapstructure"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/bitbucketcloud"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/bitbucketcloud/types"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/payload"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"
)

func MakePR(t *testing.T, bprovider bitbucketcloud.Provider, runcnx *params.Run, bcrepo *bitbucket.Repository, opts options.E2E, title, targetNS, targetRefName string) (*types.PullRequest, *bitbucket.RepositoryBranch) {
	commitAuthor := "OpenShift Pipelines E2E test"
	commitEmail := "e2e-pipelines@redhat.com"

	entries, err := payload.GetEntries(
		map[string]string{".tekton/pipelinerun.yaml": "testdata/pipelinerun.yaml"},
		targetNS, options.MainBranch, triggertype.PullRequest.String(), map[string]string{})
	assert.NilError(t, err)
	tmpfile := fs.NewFile(t, "pipelinerun", fs.WithContent(entries[".tekton/pipelinerun.yaml"]))
	defer tmpfile.Remove()

	err = bprovider.Client.Workspaces.Repositories.Repository.WriteFileBlob(&bitbucket.RepositoryBlobWriteOptions{
		Owner:    opts.Organization,
		RepoSlug: opts.Repo,
		Files: []bitbucket.File{
			{
				Name: ".tekton/sub/dir/pr.yaml",
				Path: tmpfile.Path(),
			},
		},
		Message: title,
		Branch:  targetRefName,
		Author:  fmt.Sprintf("%s <%s>", commitAuthor, commitEmail),
	})
	assert.NilError(t, err)
	runcnx.Clients.Log.Infof("Using repo %s branch %s", bcrepo.Full_name, targetRefName)

	repobranch, err := bprovider.Client.Repositories.Repository.GetBranch(&bitbucket.RepositoryBranchOptions{
		Owner:      opts.Organization,
		RepoSlug:   opts.Repo,
		BranchName: targetRefName,
	})
	assert.NilError(t, err)

	intf, err := bprovider.Client.Repositories.PullRequests.Create(&bitbucket.PullRequestsOptions{
		Owner:        opts.Organization,
		RepoSlug:     opts.Repo,
		Title:        title,
		Message:      "A new PR for testing",
		SourceBranch: targetRefName,
	})
	assert.NilError(t, err)

	pr := &types.PullRequest{}
	err = mapstructure.Decode(intf, pr)
	assert.NilError(t, err)
	runcnx.Clients.Log.Infof("Created PR %s", pr.Links.HTML.HRef)

	return pr, repobranch
}
