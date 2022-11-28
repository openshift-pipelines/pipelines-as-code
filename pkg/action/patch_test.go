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
	pipelinev1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestPatchPipelineRun(t *testing.T) {
	ctx, _ := rtesting.SetupFakeContext(t)
	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()

	testPR := tektontest.MakePR("namespace", "force-me", map[string]*pipelinev1beta1.PipelineRunTaskRunStatus{
		"first":  tektontest.MakePrTrStatus("first", 5),
		"last":   tektontest.MakePrTrStatus("last", 15),
		"middle": tektontest.MakePrTrStatus("middle", 10),
	}, nil)

	tdata := testclient.Data{
		PipelineRuns: []*pipelinev1beta1.PipelineRun{testPR},
	}

	stdata, _ := testclient.SeedTestData(t, ctx, tdata)
	fakeClients := clients.Clients{
		Tekton:    stdata.Pipeline,
		ConsoleUI: &consoleui.TektonDashboard{BaseURL: "https://localhost.console"},
	}

	err := PatchPipelineRun(ctx, logger, fakeClients.Tekton, testPR, getLogURLMergePatch(fakeClients, testPR))
	assert.NilError(t, err)

	pr, err := fakeClients.Tekton.TektonV1beta1().PipelineRuns("namespace").Get(ctx, "force-me", metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, pr.Annotations[filepath.Join(apipac.GroupName, "log-url")], "https://localhost.console/#/namespaces/namespace/pipelineruns/force-me")
}

func getLogURLMergePatch(clients clients.Clients, pr *pipelinev1beta1.PipelineRun) map[string]interface{} {
	return map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]string{
				keys.LogURL: clients.ConsoleUI.DetailURL(pr.GetNamespace(), pr.GetName()),
			},
		},
	}
}
