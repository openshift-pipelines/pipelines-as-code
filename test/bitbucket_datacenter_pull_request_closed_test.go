package test

import (
	"context"
	"fmt"
	"testing"

	"github.com/jenkins-x/go-scm/scm"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/bitbucketdatacenter"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/payload"
	twait "github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	"github.com/tektoncd/pipeline/pkg/names"
	"gotest.tools/v3/assert"
)

func TestBitbucketDataCenterPullRequestClosed(t *testing.T) {
	targetNS := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-ns")
	ctx := context.Background()
	ctx, runcnx, opts, bbdcprovider, err := bitbucketdatacenter.Setup(ctx)
	assert.NilError(t, err)

	repo := bitbucketdatacenter.CreateCRD(ctx, t, bbdcprovider, runcnx, fmt.Sprintf("%s/%s", opts.Organization, opts.Repo), targetNS)

	entries, err := payload.GetEntries(map[string]string{
		".tekton/pr-closed.yaml": "testdata/pipelinerun-on-pull-request-closed-bitbucket-datacenter.yaml",
	}, targetNS, repo.Branch, triggertype.PullRequest.String(), map[string]string{})
	assert.NilError(t, err)

	targetRefName := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-test")
	pr := bitbucketdatacenter.CreatePR(ctx, t, bbdcprovider, runcnx, opts, repo, entries, fmt.Sprintf("%s/%s", opts.Organization, opts.Repo), targetRefName)
	runcnx.Clients.Log.Infof("PullRequest %s has been created", pr.Title)

	defer bitbucketdatacenter.TearDown(ctx, t, runcnx, bbdcprovider, pr, fmt.Sprintf("%s/%s", opts.Organization, opts.Repo), targetRefName)
	defer bitbucketdatacenter.TearDownNs(ctx, t, runcnx, targetNS)

	runcnx.Clients.Log.Infof("Merging PR %d", pr.Number)
	_, err = bbdcprovider.PullRequests.Merge(ctx, fmt.Sprintf("%s/%s", opts.Organization, opts.Repo), pr.Number, &scm.PullRequestMergeOptions{})
	assert.NilError(t, err)

	sopt := twait.SuccessOpt{
		Title:           pr.Title,
		OnEvent:         "pull_request",
		TargetNS:        targetNS,
		NumberofPRMatch: 1,
	}
	twait.Succeeded(ctx, t, runcnx, opts, sopt)
}
