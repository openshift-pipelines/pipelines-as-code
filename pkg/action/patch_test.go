package action

import (
	"path/filepath"
	"testing"

	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/consoleui"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	tektontest "github.com/openshift-pipelines/pipelines-as-code/pkg/test/tekton"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestPatchPipelineRun(t *testing.T) {
	ctx, _ := rtesting.SetupFakeContext(t)
	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()

	testPR := tektontest.MakePRStatus("namespace", "force-me", []pipelinev1.ChildStatusReference{
		tektontest.MakeChildStatusReference("first"),
		tektontest.MakeChildStatusReference("last"),
		tektontest.MakeChildStatusReference("middle"),
	}, nil)

	tdata := testclient.Data{
		PipelineRuns: []*pipelinev1.PipelineRun{testPR},
	}

	stdata, _ := testclient.SeedTestData(t, ctx, tdata)
	fakeClients := clients.Clients{
		Tekton:    stdata.Pipeline,
		ConsoleUI: &consoleui.TektonDashboard{BaseURL: "https://localhost.console"},
	}

	patchedPR, err := PatchPipelineRun(ctx, logger, "log URL", fakeClients.Tekton, testPR, getLogURLMergePatch(fakeClients, testPR))
	assert.NilError(t, err)
	assert.Equal(t, patchedPR.Annotations[filepath.Join(apipac.GroupName, "log-url")], "https://localhost.console/#/namespaces/namespace/pipelineruns/force-me")
}

func getLogURLMergePatch(clients clients.Clients, pr *pipelinev1.PipelineRun) map[string]interface{} {
	return map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]string{
				keys.LogURL: clients.ConsoleUI.DetailURL(pr),
			},
		},
	}
}
