package matcher

import (
	"log"
	"os"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
	httptesthelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test/http"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/test/provider"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/env"
	"gotest.tools/v3/fs"
	k8scheme "k8s.io/client-go/kubernetes/scheme"
	rtesting "knative.dev/pkg/reconciler/testing"
)

const (
	testHubURL         = "https://mybelovedhub"
	testCatalogHubName = "tekton"
	simplePipeline     = `---
apiVersion: tekton.dev/v1beta1
kind: Pipeline
metadata:
  name: "pipeline"
spec:
  tasks:
    - name: task
      taskSpec:
        steps:
          - name: task
            image: registry.access.redhat.com/ubi9/ubi-micro
            command: ["/bin/echo", "HELLOMOTO"]`
	simpleTask = `---
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
	tests := []struct {
		annotations            map[string]string
		filesInsideRepo        map[string]string
		gotTaskName            string
		name                   string
		remoteURLS             map[string]map[string]string
		runevent               info.Event
		wantErr                string
		wantLog                string
		wantProviderRemoteTask bool
	}{
		{
			name: "test-annotations-error-remote-http-not-k8",
			annotations: map[string]string{
				pipelinesascode.GroupName + "/task": "[http://remote.task]",
			},
			remoteURLS: map[string]map[string]string{
				"http://remote.task": {
					"body": "",
					"code": "200",
				},
			},
			wantErr: "returning empty",
		},
		{
			name: "test-good-coming-from-provider",
			annotations: map[string]string{
				pipelinesascode.GroupName + "/task": "http://provider/remote.task",
			},
			wantProviderRemoteTask: true,
			wantErr:                "returning empty",
		},
		{
			name: "test-bad-coming-from-provider",
			annotations: map[string]string{
				pipelinesascode.GroupName + "/task": "http://provider/remote.task",
			},
			wantProviderRemoteTask: false,
			wantErr:                "error getting remote task",
		},
		{
			name: "test-annotations-remote-http",
			annotations: map[string]string{
				pipelinesascode.GroupName + "/task": "[http://remote.task]",
			},
			gotTaskName: "task",
			remoteURLS: map[string]map[string]string{
				"http://remote.task": {
					"body": simpleTask,
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
					"body": simpleTask,
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
			filesInsideRepo: map[string]string{
				"be/healthy": simpleTask,
			},
			runevent: info.Event{
				SHA: "007",
			},
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
					"body": simpleTask,
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
			wantErr: "could not find",
			runevent: info.Event{
				SHA: "007",
			},
		},
		{
			name: "test-annotations-remote-no-event-not-found-no-error",
			annotations: map[string]string{
				pipelinesascode.GroupName + "/task": "[not/here]",
			},
			wantLog: "could not find remote task not/here",
			wantErr: "returning empty",
		},
		{
			name:        "test-get-from-hub-latest",
			gotTaskName: "task",
			annotations: map[string]string{
				pipelinesascode.GroupName + "/task": "[chmouzie]",
			},
			remoteURLS: map[string]map[string]string{
				testHubURL + "/resource/" + testCatalogHubName + "/task/chmouzie": {
					"body": `{"data": {"LatestVersion": {"RawURL": "http://simple.task"}}}`,
					"code": "200",
				},
				"http://simple.task": {
					"body": simpleTask,
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
				testHubURL + "/resource/" + testCatalogHubName + "/task/chmouzie/0.2": {
					"body": `{"data": {"RawURL": "http://simple.task"}}`,
					"code": "200",
				},
				"http://simple.task": {
					"body": simpleTask,
					"code": "200",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpTestClient := httptesthelper.MakeHTTPTestClient(t, tt.remoteURLS)
			observer, fakelog := zapobserver.New(zap.InfoLevel)
			logger := zap.New(observer).Sugar()
			cs := &params.Run{
				Clients: clients.Clients{
					HTTP: *httpTestClient,
					Log:  logger,
				},
				Info: info.Info{
					Pac: &info.PacOpts{
						Settings: &settings.Settings{
							HubURL:         testHubURL,
							HubCatalogName: testCatalogHubName,
						},
					},
				},
			}
			ctx, _ := rtesting.SetupFakeContext(t)
			rt := RemoteTasks{
				Run:    cs,
				Logger: logger,
				ProviderInterface: &provider.TestProviderImp{
					FilesInsideRepo:        tt.filesInsideRepo,
					WantProviderRemoteTask: tt.wantProviderRemoteTask,
				},
				Event: &tt.runevent,
			}

			got, err := rt.GetTaskFromAnnotations(ctx, tt.annotations)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr, "We should have get an error with %v but we didn't", tt.wantErr)
				return
			}
			if tt.wantLog != "" {
				assert.Assert(t, len(fakelog.FilterMessageSnippet(tt.wantLog).TakeAll()) > 0, "could not find log message: got ", fakelog)
			}
			assert.NilError(t, err, "GetTaskFromAnnotations() error = %v, wantErr %v", err, tt.wantErr)
			assert.Assert(t, len(got) > 0, "GetTaskFromAnnotations() error no tasks has been processed")

			if tt.gotTaskName != "" {
				assert.Equal(t, tt.gotTaskName, got[0].GetName())
			}
		})
	}
}

func TestGetPipelineFromAnnotations(t *testing.T) {
	tests := []struct {
		annotations     map[string]string
		filesInsideRepo map[string]string
		gotPipelineName string
		name            string
		remoteURLS      map[string]map[string]string
		runevent        info.Event
		wantErr         string
		wantLog         string
	}{
		{
			name:            "good/fetching from remote http",
			gotPipelineName: "pipeline",
			annotations: map[string]string{
				pipelinesascode.GroupName + "/pipeline": "[http://remote.pipeline]",
			},
			remoteURLS: map[string]map[string]string{
				"http://remote.pipeline": {
					"body": simplePipeline,
					"code": "200",
				},
			},
		},
		{
			name: "bad/error getting pipeline",
			annotations: map[string]string{
				pipelinesascode.GroupName + "/pipeline": "[http://remote.pipeline]",
			},
			remoteURLS: map[string]map[string]string{
				"http://remote.pipeline": {
					"code": "501",
				},
			},
			wantErr: "error getting remote pipeline",
		},
		{
			name: "bad/not a pipeline",
			annotations: map[string]string{
				pipelinesascode.GroupName + "/pipeline": "[http://remote.pipeline]",
			},
			remoteURLS: map[string]map[string]string{
				"http://remote.pipeline": {
					"body": simpleTask,
					"code": "200",
				},
			},
			wantErr: "this doesn't seem to be a proper pipeline",
		},
		{
			name: "bad/could not get remote",
			annotations: map[string]string{
				pipelinesascode.GroupName + "/pipeline": "[http://nowhere.pipeline]",
			},
			wantErr: "error getting remote pipeline",
		},
		{
			name: "bad/returning empty",
			annotations: map[string]string{
				pipelinesascode.GroupName + "/pipeline": "[http://remote.pipeline]",
			},
			remoteURLS: map[string]map[string]string{
				"http://remote.pipeline": {
					"body": "",
					"code": "200",
				},
			},
			wantErr: "returning empty",
		},
		{
			name: "bad/more than one pipeline",
			annotations: map[string]string{
				pipelinesascode.GroupName + "/pipeline": "[http://foo.bar, http://remote.pipeline]",
			},
			wantErr: "only one pipeline is allowed on remote",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpTestClient := httptesthelper.MakeHTTPTestClient(t, tt.remoteURLS)
			observer, fakelog := zapobserver.New(zap.InfoLevel)
			logger := zap.New(observer).Sugar()
			cs := &params.Run{
				Clients: clients.Clients{
					HTTP: *httpTestClient,
					Log:  logger,
				},
				Info: info.Info{
					Pac: &info.PacOpts{
						Settings: &settings.Settings{
							HubURL:         testHubURL,
							HubCatalogName: testCatalogHubName,
						},
					},
				},
			}
			ctx, _ := rtesting.SetupFakeContext(t)
			rt := RemoteTasks{
				Run:    cs,
				Logger: logger,
				ProviderInterface: &provider.TestProviderImp{
					FilesInsideRepo: tt.filesInsideRepo,
				},
				Event: &tt.runevent,
			}

			got, err := rt.GetPipelineFromAnnotations(ctx, tt.annotations)

			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr, "We should have get an error with %v but we didn't", tt.wantErr)
				return
			}
			if tt.wantLog != "" {
				assert.Assert(t, len(fakelog.FilterMessageSnippet(tt.wantLog).TakeAll()) > 0, "could not find log message: got ", fakelog)
			}
			assert.NilError(t, err)
			assert.Assert(t, len(got) > 0, "GetPipelineFromAnnotations() error no pipelines has been processed")

			if tt.gotPipelineName != "" {
				assert.Equal(t, tt.gotPipelineName, got[0].GetName())
			}
		})
	}
}

func TestGetTaskFromLocalFS(t *testing.T) {
	content := "hellomoto"
	defer env.ChangeWorkingDir(t, fs.NewDir(t, "TestGetTaskFromLocalFS", fs.WithFile("task1", content)).Path())()
	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()
	taskContent, err := getTaskFromLocalFS("task1", logger)
	assert.NilError(t, err)
	assert.Equal(t, content, taskContent)
}
