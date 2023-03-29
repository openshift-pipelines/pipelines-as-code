//go:build e2e
// +build e2e

package test

import (
	"context"
	"testing"
	"time"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
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

func revertCM(ctx context.Context, t *testing.T, runcnx *params.Run) {
	cmList, err := runcnx.Clients.Kube.CoreV1().ConfigMaps("").List(ctx, metav1.ListOptions{})
	assert.NilError(t, err)
	for _, v := range cmList.Items {
		if v.Name == "pipelines-as-code" {
			cmData, err := runcnx.Clients.Kube.CoreV1().ConfigMaps(v.Namespace).Get(ctx, v.Name, metav1.GetOptions{})
			assert.NilError(t, err)
			delete(cmData.Data, customConsoleDashboardName)
			delete(cmData.Data, customConsoleDashboardURL)
			delete(cmData.Data, customConsoleDashboardPRDetailURL)
			delete(cmData.Data, customConsoleDashboardPRTaskLog)
			_, err = runcnx.Clients.Kube.CoreV1().ConfigMaps(v.Namespace).Update(ctx, cmData, metav1.UpdateOptions{})
			assert.NilError(t, err)
		}
	}
}

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

	prs, err := topts.Params.Clients.Tekton.TektonV1().PipelineRuns(topts.TargetNS).List(ctx, metav1.ListOptions{})

	assert.NilError(t, err)
	// asserting with non empty because we are not sure the URL for K8S and Openshift
	assert.Assert(t, prs.Items[0].Annotations["pipelinesascode.tekton.dev/log-url"] != "")

	// update config map with console dashboard information
	cmList, er := topts.Params.Clients.Kube.CoreV1().ConfigMaps("").List(ctx, metav1.ListOptions{})
	assert.NilError(t, er)
	for _, v := range cmList.Items {
		if v.Name == "pipelines-as-code" {
			cmData, err := topts.Params.Clients.Kube.CoreV1().ConfigMaps(v.Namespace).Get(ctx, v.Name, metav1.GetOptions{})
			assert.NilError(t, err)
			cmData.Data[customConsoleDashboardName] = "My custom console"
			cmData.Data[customConsoleDashboardURL] = "https://custom-console.ospqa.com"
			cmData.Data[customConsoleDashboardPRDetailURL] = "https://custom-console.ospqa.com/foo-pr"
			cmData.Data[customConsoleDashboardPRTaskLog] = "https://custom-console.ospqa.com/foo-pr/logs/task"
			_, err = topts.Params.Clients.Kube.CoreV1().ConfigMaps(v.Namespace).Update(ctx, cmData, metav1.UpdateOptions{})
			assert.NilError(t, err)
		}
	}

	tgitea.PostCommentOnPullRequest(t, topts, "/retest")
	tgitea.WaitForStatus(t, topts, topts.TargetRefName)

	waitOpts := twait.Opts{
		RepoName:        topts.TargetNS,
		Namespace:       topts.TargetNS,
		MinNumberStatus: 1, // 1 means 2 🙃
		PollTimeout:     twait.DefaultTimeout,
		TargetSHA:       topts.PullRequest.Head.Sha,
	}
	err = twait.UntilRepositoryUpdated(ctx, topts.Params.Clients, waitOpts)
	assert.NilError(t, err)

	time.Sleep(15 * time.Second) // “Evil does not sleep. It waits.” - Galadriel

	prs, err = topts.Params.Clients.Tekton.TektonV1().PipelineRuns(topts.TargetNS).List(ctx, metav1.ListOptions{})
	assert.NilError(t, err)

	assert.Equal(t, len(prs.Items), 1, "should have only one pipelinerun, but we have: %d", len(prs.Items))

	prs, err = topts.Params.Clients.Tekton.TektonV1().PipelineRuns(topts.TargetNS).List(ctx, metav1.ListOptions{})
	assert.NilError(t, err)

	logURL := map[string]string{}
	for _, v := range prs.Items {
		logURL[v.Annotations["pipelinesascode.tekton.dev/log-url"]] = v.Annotations["pipelinesascode.tekton.dev/log-url"]
	}
	// Check that latest create Pipeline Run will have the console dashboard URL
	assert.Assert(t, logURL["https://custom-console.ospqa.com/foo-pr"] == "https://custom-console.ospqa.com/foo-pr")

	// Resetting back by removing console dashboard information
	defer revertCM(ctx, t, topts.Params)
}

// Local Variables:
// compile-command: "go test -tags=e2e -v -run TestGiteaCustomConsoleDashboard$ ."
// End:
