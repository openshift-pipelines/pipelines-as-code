package consoleui

import (
	"testing"

	"gotest.tools/v3/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestFallbackConsole(t *testing.T) {
	fbc := &FallBackConsole{}
	ctx, _ := rtesting.SetupFakeContext(t)
	unsf := &unstructured.Unstructured{}
	unsf.SetUnstructuredContent(map[string]interface{}{
		"apiVersion": "foo.io/v1",
		"kind":       "Random",
	})
	dynClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), unsf)
	assert.NilError(t, fbc.UI(ctx, dynClient))
	assert.Assert(t, fbc.URL() != "")
	assert.Assert(t, fbc.DetailURL("ns", "pr") != "")
	assert.Assert(t, fbc.TaskLogURL("ns", "pr", "task") != "")
}
