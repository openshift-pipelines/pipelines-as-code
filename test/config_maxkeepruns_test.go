//go:build e2e
// +build e2e

package test

import (
	"context"
	"testing"

	ghlib "github.com/google/go-github/v50/github"
	tgithub "github.com/openshift-pipelines/pipelines-as-code/test/pkg/github"
	twait "github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGithubMaxKeepRuns(t *testing.T) {
	ctx := context.TODO()
	runcnx, ghcnx, opts, targetNS, targetRefName, prNumber, sha := tgithub.RunPullRequest(ctx, t,
		"Github MaxKeepRun config",
		[]string{"testdata/pipelinerun-max-keep-run-1.yaml"}, false)
	defer tgithub.TearDown(ctx, t, runcnx, ghcnx, prNumber, targetRefName, targetNS, opts)

	runcnx.Clients.Log.Infof("Creating /retest in PullRequest")
	_, _, err := ghcnx.Client.Issues.CreateComment(ctx,
		opts.Organization,
		opts.Repo, prNumber,
		&ghlib.IssueComment{Body: ghlib.String("/retest")})
	assert.NilError(t, err)

	runcnx.Clients.Log.Infof("Wait for the second repository update to be updated")
	waitOpts := twait.Opts{
		RepoName:        targetNS,
		Namespace:       targetNS,
		MinNumberStatus: 1,
		PollTimeout:     twait.DefaultTimeout,
		TargetSHA:       sha,
	}
	err = twait.UntilRepositoryUpdated(ctx, runcnx.Clients, waitOpts)
	assert.NilError(t, err)

	prs, err := runcnx.Clients.Tekton.TektonV1().PipelineRuns(targetNS).List(ctx, metav1.ListOptions{})
	assert.NilError(t, err)
	assert.Equal(t, len(prs.Items), 1)
}

// Local Variables:
// compile-command: "go test -tags=e2e -v -run TestMaxKeepRuns$ ."
// End:
