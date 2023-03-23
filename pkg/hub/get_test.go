package hub

import (
	"fmt"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
	httptesthelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test/http"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestGetTask(t *testing.T) {
	const testHubURL = "https://myprecioushub"
	const testCatalogHubName = "tekton"

	tests := []struct {
		name    string
		task    string
		want    string
		wantErr bool
		config  map[string]map[string]string
	}{
		{
			name:    "get-task-latest",
			task:    "task1",
			want:    "This is Task1",
			wantErr: false,
			config: map[string]map[string]string{
				fmt.Sprintf("%s/resource/%s/task/task1", testHubURL, testCatalogHubName): {
					"body": `{"data":{"latestVersion": {"rawURL": "https://get.me/task1"}}}`,
					"code": "200",
				},
				"https://get.me/task1": {
					"body": "This is Task1",
					"code": "200",
				},
			},
		},
		{
			name:    "get-latest-task-not-there",
			task:    "task1",
			wantErr: true,
			config: map[string]map[string]string{
				fmt.Sprintf("%s/resource/%s/task/task1", testHubURL, testCatalogHubName): {
					"code": "404",
				},
			},
		},
		{
			name:    "get-specific-task-not-there",
			task:    "task1:1.1",
			wantErr: true,
			config: map[string]map[string]string{
				fmt.Sprintf("%s/resource/%s/task/task1/1.1", testHubURL, testCatalogHubName): {
					"code": "404",
				},
			},
		},
		{
			name:    "get-task-specific",
			task:    "task2:1.1",
			want:    "This is Task2",
			wantErr: false,
			config: map[string]map[string]string{
				fmt.Sprintf("%s/resource/%s/task/task2/1.1", testHubURL, testCatalogHubName): {
					"body": `{"data":{"rawURL": "https://get.me/task2"}}`,
					"code": "200",
				},
				"https://get.me/task2": {
					"body": "This is Task2",
					"code": "200",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpTestClient := httptesthelper.MakeHTTPTestClient(tt.config)
			ctx, _ := rtesting.SetupFakeContext(t)
			cs := &params.Run{
				Clients: clients.Clients{
					HTTP: *httpTestClient,
				},
				Info: info.Info{Pac: &info.PacOpts{Settings: &settings.Settings{HubURL: testHubURL, HubCatalogName: testCatalogHubName}}},
			}
			got, err := GetTask(ctx, cs, tt.task)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetTask() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got != tt.want {
				t.Errorf("GetTask() got = %v, want %v", got, tt.want)
			}
		})
	}
}
