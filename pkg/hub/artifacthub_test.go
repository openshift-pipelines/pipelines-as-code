// Copyright © 2022 The Tekton Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hub

import (
	"context"
	"fmt"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	httptesthelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test/http"
	"gotest.tools/v3/assert"
)

// // Mock transport for HTTP client
// type mockRoundTripper struct {
// 	roundTripFunc func(req *http.Request) (*http.Response, error)
// }

// func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
// 	return m.roundTripFunc(req)
// }

// // Create a mock client that returns a specified response for test cases
// func newMockHTTPClient(fn func(req *http.Request) (*http.Response, error)) http.Client {
// 	return http.Client{
// 		Transport: &mockRoundTripper{roundTripFunc: fn},
// 	}
// }

func TestNewArtifactHubClient(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		wantedURL string
	}{
		{
			name:      "URL with api/v1 suffix",
			url:       "https://artifacthub.io/api/v1",
			wantedURL: "https://artifacthub.io/api/v1",
		},
		{
			name:      "URL without api/v1 suffix",
			url:       "https://artifacthub.io",
			wantedURL: "https://artifacthub.io/api/v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newArtifactHubClient(&params.Run{}, tt.url, "test")
			ahClient, ok := client.(*artifactHubClient)
			assert.Assert(t, ok)
			assert.Equal(t, tt.wantedURL, ahClient.url)
		})
	}
}

func TestGetTypeByKind(t *testing.T) {
	tests := []struct {
		name        string
		catalogName string
		kind        string
		wantType    string
		wantCatalog string
	}{
		{
			name:        "task with default catalog",
			catalogName: "default",
			kind:        "task",
			wantType:    artifactHubTaskType,
			wantCatalog: defaultArtifactHubCatalogTaskName,
		},
		{
			name:        "task with empty catalog",
			catalogName: "",
			kind:        "task",
			wantType:    artifactHubTaskType,
			wantCatalog: defaultArtifactHubCatalogTaskName,
		},
		{
			name:        "task with custom catalog",
			catalogName: "custom",
			kind:        "task",
			wantType:    artifactHubTaskType,
			wantCatalog: "custom",
		},
		{
			name:        "pipeline with default catalog",
			catalogName: "default",
			kind:        "pipeline",
			wantType:    artifactHubPipelineType,
			wantCatalog: defaultArtifactHubCatalogPipelineName,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotCatalog := getArtifactHubTypeByKind(tt.catalogName, tt.kind)
			assert.Equal(t, tt.wantType, gotType)
			assert.Equal(t, tt.wantCatalog, gotCatalog)
		})
	}
}

func TestArtifactHubClient_GetResource(t *testing.T) {
	ctx := context.Background()
	testCases := []struct {
		name        string
		resource    string
		kind        string
		catalogName string
		want        string
		config      map[string]map[string]string
		wantErr     bool
	}{
		{
			name:        "get latest version",
			resource:    "git-clone",
			kind:        "task",
			catalogName: "tekton",
			want:        "atask",
			config: map[string]map[string]string{
				fmt.Sprintf("%s/api/v1/packages/tekton-task/%s/git-clone", testHubURL, testCatalogHubName): {
					"body": fmt.Sprintf(sampleArtifactHubManifest, "atask"),
					"code": "200",
				},
			},
			wantErr: false,
		},
		{
			name:        "get specific version",
			resource:    "git-clone:0.9.0",
			kind:        "task",
			catalogName: "tekton",
			want:        "atask",
			wantErr:     false,
			config: map[string]map[string]string{
				fmt.Sprintf("%s/api/v1/packages/tekton-task/%s/git-clone/0.9.0", testHubURL, testCatalogHubName): {
					"body": fmt.Sprintf(sampleArtifactHubManifest, "atask"),
					"code": "200",
				},
			},
		},
		{
			name:        "invalid response",
			resource:    "git-clone",
			kind:        "task",
			catalogName: "tekton",
			want:        "",
			wantErr:     true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			httpTestClient := httptesthelper.MakeHTTPTestClient(tc.config)

			cs := &params.Run{
				Clients: clients.Clients{
					HTTP: *httpTestClient,
				},
			}
			client := newArtifactHubClient(cs, testHubURL, testCatalogHubName)
			got, err := client.GetResource(ctx, tc.catalogName, tc.resource, tc.kind)

			if tc.wantErr {
				assert.Assert(t, err != nil)
			} else {
				assert.NilError(t, err)
				assert.Equal(t, tc.want, got)
			}
		})
	}
}
