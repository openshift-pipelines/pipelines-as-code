//go:build e2e
// +build e2e

package test

import (
	"context"
	"testing"
	"time"

	ghlib "github.com/google/go-github/v56/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	tgithub "github.com/openshift-pipelines/pipelines-as-code/test/pkg/github"
	twait "github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func TestGithubMaxKeepRuns(t *testing.T) {
	ctx := context.TODO()
	runcnx, ghcnx, opts, targetNS, targetRefName, prNumber, sha := tgithub.RunPullRequest(ctx, t,
		"Github MaxKeepRun config",
		[]string{"testdata/pipelinerun-max-keep-run-1.yaml"}, false, false)
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
		MinNumberStatus: 2,
		PollTimeout:     twait.DefaultTimeout,
		TargetSHA:       sha,
	}
	err = twait.UntilRepositoryUpdated(ctx, runcnx.Clients, waitOpts)
	assert.NilError(t, err)

	count := 0
	for {
		prs, err := runcnx.Clients.Tekton.TektonV1().PipelineRuns(targetNS).List(ctx, metav1.ListOptions{})
		if err == nil && len(prs.Items) == 1 {
			if prs.Items[0].GetStatusCondition().GetCondition(apis.ConditionSucceeded).GetReason() == "Running" {
				t.Logf("skipping %s since currently running", prs.Items[0].GetName())
				continue
			}
			// making sure secret is not deleted for existing pipelinerun
			if secretName, ok := prs.Items[0].GetAnnotations()[keys.GitAuthSecret]; ok {
				sData, err := runcnx.Clients.Kube.CoreV1().Secrets(targetNS).Get(ctx, secretName, metav1.GetOptions{})
				assert.NilError(t, err, "Secret should not have been deleted while running pipelinerun")
				assert.Assert(t, sData.Name != "")
			}
			break
		}
		time.Sleep(10 * time.Second)
		if count > 10 {
			t.Fatalf("PipelineRun cleanups has not been done, we found %d in %s", len(prs.Items), targetNS)
		}
		count++
	}
}

// Local Variables:
// compile-command: "go test -tags=e2e -v -run ^TestGithubMaxKeepRuns$"
// End:
