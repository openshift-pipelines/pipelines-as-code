package config

import (
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"os"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	ghtesthelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test/github"
	httptesthelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test/http"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"gotest.tools/v3/assert"
	k8scheme "k8s.io/client-go/kubernetes/scheme"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestMain(m *testing.M) {
	s := k8scheme.Scheme
	if err := tektonv1beta1.AddToScheme(s); err != nil {
		log.Fatalf("Unable to add route scheme: (%v)", err)
	}
	ret := m.Run()
	os.Exit(ret)
}

func TestRemoteTasksGetTaskFromAnnotations(t *testing.T) {
	simpletask := `---
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: task
spec:
  steps:
    - name: task-step
      image: image
      script: |
       echo hello`

	fakeGHclient, mux, _, ghTeardown := ghtesthelper.SetupGH()
	defer ghTeardown()

	mux.HandleFunc("/repos/contents/be/healthy", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{
  "name": "healthy",
  "path": "be/healthy",
  "sha": "takepicture"}`)
	})
	mux.HandleFunc("/repos/git/blobs/takepicture", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{
  "name": "healthy",
  "path": "be/healthy",
  "sha": "takepicture",
  "size": 68,
  "content": "%s\n",
  "encoding": "base64"}`,
			base64.StdEncoding.EncodeToString([]byte(simpletask)))
	})
	mux.HandleFunc("/repos/contents/pas/la", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	tests := []struct {
		name        string
		runinfo     *webvcs.RunInfo
		annotations map[string]string
		wantErr     string
		gotTaskName string
		remoteURLS  map[string]map[string]string
	}{
		{
			name: "test-annotations-error-remote-http-not-k8",
			annotations: map[string]string{
				pipelinesascode.GroupName + "/task": "[http://remote.task]",
			},
			remoteURLS: map[string]map[string]string{
				"https://remote.task": {
					"body": "",
					"code": "200",
				},
			},
			wantErr: "not looking like a kubernetes resource",
		},
		{
			name: "test-annotations-remote-http",
			annotations: map[string]string{
				pipelinesascode.GroupName + "/task": "[http://remote.task]",
			},
			gotTaskName: "task",
			remoteURLS: map[string]map[string]string{
				"http://remote.task": {
					"body": simpletask,
					"code": "200",
				},
			},
		},
		{
			name: "test-annotations-remote-https",
			annotations: map[string]string{
				pipelinesascode.GroupName + "/task": "[https://remote.task]",
			},
			gotTaskName: "task",
			remoteURLS: map[string]map[string]string{
				"https://remote.task": {
					"body": simpletask,
					"code": "200",
				},
			},
		},
		{
			name: "test-annotations-inside-repo",
			annotations: map[string]string{
				pipelinesascode.GroupName + "/task": "[be/healthy]",
			},
			gotTaskName: "task",
			runinfo:     &webvcs.RunInfo{},
		},
		{
			name: "test-annotations-remote-http-skipping-notmatching",
			annotations: map[string]string{
				pipelinesascode.GroupName + "/task":  "[http://remote.task]",
				pipelinesascode.GroupName + "/taskA": "[http://other.task]", // That's wrong this would be skipped
			},
			gotTaskName: "task",
			remoteURLS: map[string]map[string]string{
				"http://remote.task": {
					"body": simpletask,
					"code": "200",
				},
			},
		},
		{
			name: "test-annotations-remote-http-bad-annotation",
			annotations: map[string]string{
				pipelinesascode.GroupName + "/task": "[http://remote.task",
			},
			wantErr: "annotations in pipeline are in wrong format",
		},
		{
			name: "test-annotations-remote-inside-file-not-found",
			annotations: map[string]string{
				pipelinesascode.GroupName + "/task": "[pas/la]",
			},
			runinfo: &webvcs.RunInfo{},
			wantErr: "404",
		},
		{
			name:        "test-get-from-hub-latest",
			gotTaskName: "task",
			annotations: map[string]string{
				pipelinesascode.GroupName + "/task": "[chmouzie]",
			},
			remoteURLS: map[string]map[string]string{
				"https://api.hub.tekton.dev/v1/resource/tekton/task/chmouzie": {
					"body": `{"data": {"LatestVersion": {"RawURL": "http://simple.task"}}}`,
					"code": "200",
				},
				"http://simple.task": {
					"body": simpletask,
					"code": "200",
				},
			},
		},
		{
			name:        "test-get-from-hub-specific-version",
			gotTaskName: "task",
			annotations: map[string]string{
				pipelinesascode.GroupName + "/task": "[chmouzie:0.2]",
			},
			remoteURLS: map[string]map[string]string{
				"https://api.hub.tekton.dev/v1/resource/tekton/task/chmouzie/0.2": {
					"body": `{"data": {"RawURL": "http://simple.task"}}`,
					"code": "200",
				},
				"http://simple.task": {
					"body": simpletask,
					"code": "200",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpTestClient := httptesthelper.MakeHTTPTestClient(t, tt.remoteURLS)
			cs := &cli.Clients{
				HTTPClient: *httpTestClient,
				GithubClient: webvcs.GithubVCS{
					Client: fakeGHclient,
				},
			}
			ctx, _ := rtesting.SetupFakeContext(t)
			rt := RemoteTasks{
				Clients: cs,
				Runinfo: tt.runinfo,
			}
			got, err := rt.GetTaskFromAnnotations(ctx, tt.annotations)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr, "We should have get an error with %v but we didn't", tt.wantErr)
				return
			}

			assert.NilError(t, err, "GetTaskFromAnnotations() error = %v, wantErr %v", err, tt.wantErr)
			assert.Assert(t, len(got) > 0, "GetTaskFromAnnotations() error no tasks has been processed")

			if tt.gotTaskName != "" {
				assert.Equal(t, tt.gotTaskName, got[0].GetName())
			}
		})
	}
}
