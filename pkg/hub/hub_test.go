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
	"sync"
	"testing"

	hubtypes "github.com/openshift-pipelines/pipelines-as-code/pkg/hub/vars"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
	"gotest.tools/v3/assert"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name        string
		catalogName string
		catalogType string
		wantType    string
		wantErr     bool
	}{
		{
			name:        "tektonhub client",
			catalogName: "tekton",
			catalogType: hubtypes.TektonHubType,
			wantType:    "*hub.tektonHubClient",
			wantErr:     false,
		},
		{
			name:        "artifacthub client",
			catalogName: "artifact",
			catalogType: hubtypes.ArtifactHubType,
			wantType:    "*hub.artifactHubClient",
			wantErr:     false,
		},
		{
			name:        "default to artifacthub client if type is empty",
			catalogName: "default",
			catalogType: "",
			wantType:    "*hub.artifactHubClient",
			wantErr:     false,
		},
		{
			name:        "error on invalid catalog name",
			catalogName: "invalid",
			catalogType: "",
			wantType:    "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Setup catalogs
			catalogs := &sync.Map{}
			if tt.catalogName != "invalid" {
				catalogs.Store(tt.catalogName, settings.HubCatalog{
					Name: tt.catalogName,
					URL:  "https://test.com",
					Type: tt.catalogType,
				})
			}

			// Create run with HubCatalogs
			pacOpts := info.NewPacOpts()
			pacOpts.HubCatalogs = catalogs

			cs := &params.Run{
				Info: info.Info{
					Pac: pacOpts,
				},
			}

			client, err := NewClient(ctx, cs, tt.catalogName)

			if tt.wantErr {
				assert.Assert(t, err != nil, "expected error but got nil")
			} else {
				assert.NilError(t, err)

				// Check client type without using fmt.Sprintf
				switch tt.wantType {
				case "*hub.tektonHubClient":
					_, ok := client.(*tektonHubClient)
					assert.Assert(t, ok, "expected *tektonHubClient but got different type")
				case "*hub.artifactHubClient":
					_, ok := client.(*artifactHubClient)
					assert.Assert(t, ok, "expected *artifactHubClient but got different type")
				}
			}
		})
	}
}
