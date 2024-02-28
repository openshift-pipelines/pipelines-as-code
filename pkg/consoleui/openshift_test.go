package consoleui

import (
	"testing"

	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestOpenshiftConsoleUI(t *testing.T) {
	fakeroute := &unstructured.Unstructured{}
	fakeroute.SetUnstructuredContent(map[string]interface{}{
		"apiVersion": "route.openshift.io/v1",
		"kind":       "Route",
		"metadata": map[string]interface{}{
			"name":      "console",
			"namespace": "openshift-console",
		},
		"spec": map[string]interface{}{
			"host": "http://fakeconsole",
		},
	})
	emptys := &unstructured.Unstructured{}
	emptys.SetUnstructuredContent(map[string]interface{}{
		"apiVersion": "route.openshift.io/v1",
		"kind":       "Route",
		"metadata": map[string]interface{}{
			"name":      "not",
			"namespace": "console",
		},
	})
	nospeccontent := &unstructured.Unstructured{}
	nospeccontent.SetUnstructuredContent(map[string]interface{}{
		"apiVersion": "route.openshift.io/v1",
		"kind":       "Route",
		"metadata": map[string]interface{}{
			"name":      "console",
			"namespace": "openshift-console",
		},
		"spec": map[string]interface{}{},
	})

	nohost := &unstructured.Unstructured{}
	nohost.SetUnstructuredContent(map[string]interface{}{
		"apiVersion": "route.openshift.io/v1",
		"kind":       "Route",
		"metadata": map[string]interface{}{
			"name":      "console",
			"namespace": "openshift-console",
		},
	})

	tests := []struct {
		name     string
		unsf     *unstructured.Unstructured
		wantErr  bool
		wantHost string
		wantName string
	}{
		{
			name:     "Get Openshift Console url",
			unsf:     fakeroute,
			wantErr:  false,
			wantHost: "http://fakeconsole",
		},
		{
			name:     "Get Openshift Console name",
			unsf:     fakeroute,
			wantErr:  false,
			wantName: "OpenShift Console",
		},
		{
			name:    "No Openshift Console",
			unsf:    emptys,
			wantErr: true,
		},
		{
			name:    "No spec in route",
			unsf:    nospeccontent,
			wantErr: true,
		},
		{
			name:    "No host in route.spec",
			unsf:    nohost,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			o := &OpenshiftConsole{}
			dynClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), tt.unsf)
			if err := o.UI(ctx, dynClient); (err != nil) != tt.wantErr {
				t.Fatalf("UI() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantHost != "" {
				if o.host != tt.wantHost {
					t.Fatalf("UI() host = %v, want %v", o.host, tt.wantHost)
				}
			}
			if tt.wantName != "" {
				if o.GetName() != tt.wantName {
					t.Fatalf("UI() name = %v, want %v", o.GetName(), tt.wantName)
				}
			}
		})
	}
}

func TestOpenshiftConsoleURLs(t *testing.T) {
	pr := &tektonv1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "theNS",
			Name:      "pr",
		},
	}
	trStatus := &tektonv1.PipelineRunTaskRunStatus{
		PipelineTaskName: "task",
	}
	o := OpenshiftConsole{host: "fakeconsole"}
	assert.Equal(t, o.URL(), "https://fakeconsole")
	assert.Equal(t, o.DetailURL(pr), "https://fakeconsole/k8s/ns/theNS/tekton.dev~v1~PipelineRun/pr")
	assert.Equal(t, o.TaskLogURL(pr, trStatus), "https://fakeconsole/k8s/ns/theNS/tekton.dev~v1~PipelineRun/pr/logs/task")
	assert.Equal(t, o.NamespaceURL(pr), "https://fakeconsole/pipelines/ns/theNS/pipeline-runs")
}
