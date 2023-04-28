//go:build e2e
// +build e2e

package test

import (
	"context"
	"testing"
	"time"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/configmap"
	tgitea "github.com/openshift-pipelines/pipelines-as-code/test/pkg/gitea"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	twait "github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	customConsoleDashboardName        = "custom-console-name"
	customConsoleDashboardURL         = "custom-console-url"
	customConsoleDashboardPRDetailURL = "custom-console-url-pr-details"
	customConsoleDashboardPRTaskLog   = "custom-console-url-pr-tasklog"
)

func TestGiteaCustomConsoleDashboard(t *testing.T) {
	ctx := context.TODO()
	topts := &tgitea.TestOpts{
		Regexp:      successRegexp,
		TargetEvent: options.PullRequestEvent,
		YAMLFiles: map[string]string{
			".tekton/pr.yaml": "testdata/pipelinerun-max-keep-run-1.yaml",
		},
		NoCleanup:      true,
		CheckForStatus: "success",
		ExpectEvents:   false,
	}
	defer tgitea.TestPR(t, topts)()

	prs, err := topts.ParamsRun.Clients.Tekton.TektonV1().PipelineRuns(topts.TargetNS).List(ctx, metav1.ListOptions{})

	assert.NilError(t, err)
	// asserting with non empty because we are not sure the URL for K8S and Openshift
	assert.Assert(t, prs.Items[0].Annotations[keys.LogURL] != "")

	data := map[string]string{
		customConsoleDashboardName:        "My custom console",
		customConsoleDashboardURL:         "https://custom-console.test.com",
		customConsoleDashboardPRDetailURL: "https://custom-console.test.com/foo-pr",
		customConsoleDashboardPRTaskLog:   "https://custom-console.test.com/foo-pr/logs/task",
	}
	defer configmap.ChangeGlobalConfig(ctx, t, topts.ParamsRun, data)()

	tgitea.PostCommentOnPullRequest(t, topts, "/retest")
	tgitea.WaitForStatus(t, topts, topts.TargetRefName)

	waitOpts := twait.Opts{
		RepoName:        topts.TargetNS,
		Namespace:       topts.TargetNS,
		MinNumberStatus: 1, // 1 means 2 🙃
		PollTimeout:     twait.DefaultTimeout,
		TargetSHA:       topts.PullRequest.Head.Sha,
	}
	err = twait.UntilRepositoryUpdated(ctx, topts.ParamsRun.Clients, waitOpts)
	assert.NilError(t, err)

	time.Sleep(15 * time.Second) // “Evil does not sleep. It waits.” - Galadriel

	prs, err = topts.ParamsRun.Clients.Tekton.TektonV1().PipelineRuns(topts.TargetNS).List(ctx, metav1.ListOptions{})
	assert.NilError(t, err)

	assert.Equal(t, len(prs.Items), 1, "should have only one pipelinerun, but we have: %d", len(prs.Items))

	prs, err = topts.ParamsRun.Clients.Tekton.TektonV1().PipelineRuns(topts.TargetNS).List(ctx, metav1.ListOptions{})
	assert.NilError(t, err)

	logURL := map[string]string{}
	for _, v := range prs.Items {
		logURL[v.Annotations[keys.LogURL]] = v.Annotations[keys.LogURL]
	}
	// Check that latest create Pipeline Run will have the console dashboard URL
	assert.Assert(t, logURL["https://custom-console.test.com/foo-pr"] == "https://custom-console.test.com/foo-pr")
}

// Local Variables:
// compile-command: "go test -tags=e2e -v -run TestGiteaCustomConsoleDashboard$ ."
// End:
