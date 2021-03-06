package consoleui

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestTektonDashboard(t *testing.T) {
	tr := &TektonDashboard{
		BaseURL: "https://test",
	}
	ctx, _ := rtesting.SetupFakeContext(t)

	unsf := &unstructured.Unstructured{}
	unsf.SetUnstructuredContent(map[string]interface{}{
		"apiVersion": "foo.io/v1",
		"kind":       "Random",
	})
	dynClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), unsf)
	assert.NilError(t, tr.UI(ctx, dynClient))
	assert.Assert(t, strings.Contains(tr.DetailURL("ns", "pr"), "namespaces/ns"))
	assert.Assert(t, strings.Contains(tr.TaskLogURL("ns", "pr", "task"), "pipelineTask=task"))
	assert.Assert(t, strings.Contains(tr.URL(), "test"))
}
