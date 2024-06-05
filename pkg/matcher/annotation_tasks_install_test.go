package matcher

import (
	"fmt"
	"log"
	"os"
	"sync"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
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
)

func TestMain(m *testing.M) {
	s := k8scheme.Scheme
	if err := tektonv1beta1.AddToScheme(s); err != nil {
		log.Fatalf("Unable to add route scheme: (%v)", err)
	}
	ret := m.Run()
	os.Exit(ret)
}

func readTDfile(t *testing.T, testname string) string {
	t.Helper()
	data, err := os.ReadFile("testdata/" + testname + ".yaml")
	assert.NilError(t, err)
	return string(data)
}

func TestRemoteTasksGetTaskFromAnnotations(t *testing.T) {
	var hubCatalogs sync.Map
	hubCatalogs.Store(
		"default", settings.HubCatalog{
			Index: "default",
			URL:   testHubURL,
			Name:  testCatalogHubName,
		})
	hubCatalogs.Store(
		"anotherHub", settings.HubCatalog{
			Index: "1",
			URL:   testHubURL,
			Name:  testCatalogHubName,
		})
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
				keys.Task: "[http://remote.task]",
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
				keys.Task: "http://provider/remote.task",
			},
			wantProviderRemoteTask: true,
			wantErr:                "returning empty",
		},
		{
			name: "test-bad-coming-from-provider",
			annotations: map[string]string{
				keys.Task: "http://provider/remote.task",
			},
			wantProviderRemoteTask: false,
			wantErr:                "error getting remote task",
		},
		{
			name: "test-annotations-remote-http",
			annotations: map[string]string{
				keys.Task: "[http://remote.task]",
			},
			remoteURLS: map[string]map[string]string{
				"http://remote.task": {
					"body": readTDfile(t, "task-good"),
					"code": "200",
				},
			},
		},
		// TODO: to uncomment in the future when fixing the Valdiate bug issue
		// {
		// 	name: "invalid-remote-task",
		// 	annotations: map[string]string{
		// 		keys.Task: "[http://remote.task]",
		// 	},
		// 	gotTaskName: "task",
		// 	remoteURLS: map[string]map[string]string{
		// 		"http://remote.task": {
		// 			"body": readTDfile(t, "task-invalid"),
		// 			"code": "200",
		// 		},
		// 	},
		// 	wantErr: "cannot be validated properly",
		// },
		{
			name: "test-annotations-remote-https",
			annotations: map[string]string{
				keys.Task: "[https://remote.task]",
			},
			gotTaskName: "task",
			remoteURLS: map[string]map[string]string{
				"https://remote.task": {
					"body": readTDfile(t, "task-good"),
					"code": "200",
				},
			},
		},
		{
			name: "test-annotations-inside-repo",
			annotations: map[string]string{
				keys.Task: "[be/healthy]",
			},
			gotTaskName: "task",
			filesInsideRepo: map[string]string{
				"be/healthy": readTDfile(t, "task-good"),
			},
			runevent: info.Event{
				SHA: "007",
			},
		},
		{
			name: "test-annotations-remote-http-skipping-notmatching",
			annotations: map[string]string{
				keys.Task:                            "[http://remote.task]",
				pipelinesascode.GroupName + "/taskA": "[http://other.task]", // That's wrong this would be skipped
			},
			gotTaskName: "task",
			remoteURLS: map[string]map[string]string{
				"http://remote.task": {
					"body": readTDfile(t, "task-good"),
					"code": "200",
				},
			},
		},
		{
			name: "test-annotations-remote-http-bad-annotation",
			annotations: map[string]string{
				keys.Task: "[http://remote.task",
			},
			wantErr: "annotations in pipeline are in wrong format",
		},
		{
			name: "test-annotations-remote-inside-file-not-found",
			annotations: map[string]string{
				keys.Task: "[pas/la]",
			},
			wantErr: "could not find",
			runevent: info.Event{
				SHA: "007",
			},
		},
		{
			name: "test-annotations-remote-no-event-not-found-no-error",
			annotations: map[string]string{
				keys.Task: "[not/here]",
			},
			wantLog: "could not find remote file not/here",
			wantErr: "returning empty",
		},
		{
			name: "test-annotations-unknown-hub",
			annotations: map[string]string{
				keys.Task: "[foo://bar]",
			},
			wantLog: "custom catalog foo is not found",
			wantErr: "could not get remote task \"foo://bar\": returning empty",
		},
		{
			name:        "test-get-from-custom-hub",
			gotTaskName: "task",
			annotations: map[string]string{
				keys.Task: "[anotherHub://chmouzie]",
			},
			wantLog: "successfully fetched task chmouzie from custom catalog HUB anotherHub on URL https://mybelovedhub",
			remoteURLS: map[string]map[string]string{
				testHubURL + "/resource/" + testCatalogHubName + "/task/chmouzie": {
					"body": `{"data": {"LatestVersion": {"version": "0.1"}}}`,
					"code": "200",
				},
				fmt.Sprintf("%s/resource/%s/task/chmouzie/0.1/raw", testHubURL, testCatalogHubName): {
					"body": readTDfile(t, "task-good"),
					"code": "200",
				},
			},
		},
		{
			name:        "test-get-from-hub-latest",
			gotTaskName: "task",
			annotations: map[string]string{
				keys.Task: "[chmouzie]",
			},
			remoteURLS: map[string]map[string]string{
				testHubURL + "/resource/" + testCatalogHubName + "/task/chmouzie": {
					"body": `{"data": {"LatestVersion": {"version": "0.1"}}}`,
					"code": "200",
				},
				fmt.Sprintf("%s/resource/%s/task/chmouzie/0.1/raw", testHubURL, testCatalogHubName): {
					"body": readTDfile(t, "task-good"),
					"code": "200",
				},
			},
		},
		{
			name:        "test-get-from-hub-specific-version",
			gotTaskName: "task",
			annotations: map[string]string{
				keys.Task: "[chmouzie:0.2]",
			},
			remoteURLS: map[string]map[string]string{
				testHubURL + "/resource/" + testCatalogHubName + "/task/chmouzie/0.2": {
					"body": `{}`,
					"code": "200",
				},
				fmt.Sprintf("%s/resource/%s/task/chmouzie/0.2/raw", testHubURL, testCatalogHubName): {
					"body": readTDfile(t, "task-good"),
					"code": "200",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpTestClient := httptesthelper.MakeHTTPTestClient(tt.remoteURLS)
			observer, fakelog := zapobserver.New(zap.InfoLevel)
			logger := zap.New(observer).Sugar()
			cs := &params.Run{
				Clients: clients.Clients{
					HTTP: *httpTestClient,
					Log:  logger,
				},
				Info: info.Info{
					Pac: &info.PacOpts{
						Settings: settings.Settings{
							HubCatalogs: &hubCatalogs,
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
			if tt.wantLog != "" {
				assert.Assert(t, len(fakelog.FilterMessageSnippet(tt.wantLog).TakeAll()) > 0, "could not find log message: got ", fakelog)
			}
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

func TestGetPipelineFromAnnotations(t *testing.T) {
	var hubCatalogs sync.Map
	hubCatalogs.Store(
		"default", settings.HubCatalog{
			Index: "default",
			URL:   testHubURL,
			Name:  testCatalogHubName,
		})
	hubCatalogs.Store(
		"anotherHub", settings.HubCatalog{
			Index: "1",
			URL:   testHubURL,
			Name:  testCatalogHubName,
		})
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
				keys.Pipeline: "[http://remote.pipeline]",
			},
			remoteURLS: map[string]map[string]string{
				"http://remote.pipeline": {
					"body": readTDfile(t, "pipeline-good"),
					"code": "200",
				},
			},
		},
		{
			name:            "good/fetching with bundle",
			gotPipelineName: "pipeline",
			annotations: map[string]string{
				keys.Pipeline: "[http://remote.pipeline]",
			},
			remoteURLS: map[string]map[string]string{
				"http://remote.pipeline": {
					"body": readTDfile(t, "pipeline-good-bundle"),
					"code": "200",
				},
			},
		},
		// TODO: to uncomment in the future when fixing the Valdiate bug issue
		// {
		// 	name: "invalid-pipeline-validaton-failed",
		// 	annotations: map[string]string{
		// 		keys.Pipeline: "[http://remote.pipeline]",
		// 	},
		// 	remoteURLS: map[string]map[string]string{
		// 		"http://remote.pipeline": {
		// 			"body": readTDfile(t, "pipeline-invalid-bundle"),
		// 			"code": "200",
		// 		},
		// 	},
		// 	wantErr: "remote pipeline from uri: http://remote.pipeline with name pipeline-test1 cannot be validated:",
		// },
		// {
		// 	name: "invalid-remote-pipeline",
		// 	annotations: map[string]string{
		// 		keys.Pipeline: "[http://remote.pipeline]",
		// 	},
		// 	remoteURLS: map[string]map[string]string{
		// 		"http://remote.pipeline": {
		// 			"body": readTDfile(t, "pipeline-invalid"),
		// 			"code": "200",
		// 		},
		// 	},
		// 	wantErr: "emote pipeline from uri: http://remote.pipeline with name pipeline cannot be validated: expected at least one, got none:",
		// },
		{
			name: "bad/error getting pipeline",
			annotations: map[string]string{
				keys.Pipeline: "[http://remote.pipeline]",
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
				keys.Pipeline: "[http://remote.pipeline]",
			},
			remoteURLS: map[string]map[string]string{
				"http://remote.pipeline": {
					"body": readTDfile(t, "task-good"),
					"code": "200",
				},
			},
			wantErr: "remote pipeline from uri: http://remote.pipeline has not been recognized as a tekton pipeline",
		},
		{
			name: "bad/could not get remote",
			annotations: map[string]string{
				keys.Pipeline: "[http://nowhere.pipeline]",
			},
			wantErr: "error getting remote pipeline",
		},
		{
			name: "bad/returning empty",
			annotations: map[string]string{
				keys.Pipeline: "[http://remote.pipeline]",
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
				keys.Pipeline: "[http://foo.bar, http://remote.pipeline]",
			},
			wantErr: "only one pipeline is allowed on remote",
		},
		{
			name: "test-annotations-unknown-hub",
			annotations: map[string]string{
				keys.Pipeline: "[foo://bar]",
			},
			wantLog: "custom catalog foo is not found",
			wantErr: "could not get remote pipeline \"foo://bar\": returning empty",
		},
		{
			name:            "test-get-from-custom-hub",
			gotPipelineName: "pipeline",
			annotations: map[string]string{
				keys.Pipeline: "[anotherHub://chmouzie]",
			},
			wantLog: "successfully fetched pipeline chmouzie from custom catalog HUB anotherHub on URL https://mybelovedhub",
			remoteURLS: map[string]map[string]string{
				testHubURL + "/resource/" + testCatalogHubName + "/pipeline/chmouzie": {
					"body": `{"data": {"LatestVersion": {"version": "0.1"}}}`,
					"code": "200",
				},
				fmt.Sprintf("%s/resource/%s/pipeline/chmouzie/0.1/raw", testHubURL, testCatalogHubName): {
					"body": readTDfile(t, "pipeline-good"),
					"code": "200",
				},
			},
		},
		{
			name:            "test-get-from-hub-latest",
			gotPipelineName: "pipeline",
			annotations: map[string]string{
				keys.Pipeline: "[chmouzie]",
			},
			remoteURLS: map[string]map[string]string{
				testHubURL + "/resource/" + testCatalogHubName + "/pipeline/chmouzie": {
					"body": `{"data": {"LatestVersion": {"version": "0.1"}}}`,
					"code": "200",
				},
				fmt.Sprintf("%s/resource/%s/pipeline/chmouzie/0.1/raw", testHubURL, testCatalogHubName): {
					"body": readTDfile(t, "pipeline-good"),
					"code": "200",
				},
			},
		},
		{
			name:            "test-get-from-hub-specific-version",
			gotPipelineName: "pipeline",
			annotations: map[string]string{
				keys.Pipeline: "[chmouzie:0.2]",
			},
			remoteURLS: map[string]map[string]string{
				testHubURL + "/resource/" + testCatalogHubName + "/pipeline/chmouzie/0.2": {
					"body": `{}`,
					"code": "200",
				},
				fmt.Sprintf("%s/resource/%s/pipeline/chmouzie/0.2/raw", testHubURL, testCatalogHubName): {
					"body": readTDfile(t, "pipeline-good"),
					"code": "200",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpTestClient := httptesthelper.MakeHTTPTestClient(tt.remoteURLS)
			observer, fakelog := zapobserver.New(zap.InfoLevel)
			logger := zap.New(observer).Sugar()

			cs := &params.Run{
				Clients: clients.Clients{
					HTTP: *httpTestClient,
					Log:  logger,
				},
				Info: info.Info{
					Pac: &info.PacOpts{
						Settings: settings.Settings{
							HubCatalogs: &hubCatalogs,
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
			assert.Assert(t, got != nil, "GetPipelineFromAnnotations() error no pipelines has been processed")

			if tt.gotPipelineName != "" {
				assert.Equal(t, tt.gotPipelineName, got.GetName())
			}
		})
	}
}

func TestGetTaskFromLocalFS(t *testing.T) {
	content := "hellomoto"
	defer env.ChangeWorkingDir(t, fs.NewDir(t, "TestGetTaskFromLocalFS", fs.WithFile("task1", content)).Path())()
	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()
	taskContent, err := getFileFromLocalFS("task1", logger)
	assert.NilError(t, err)
	assert.Equal(t, content, taskContent)
}
