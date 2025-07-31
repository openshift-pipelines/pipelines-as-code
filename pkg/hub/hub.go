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

	hubtypes "github.com/openshift-pipelines/pipelines-as-code/pkg/hub/vars"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
)

// Client is an interface for a hub client.
type Client interface {
	GetResource(ctx context.Context, catalogName, resource, kind string) (string, error)
}

// NewClient returns a new hub client.
func NewClient(_ context.Context, cs *params.Run, catalogName string) (Client, error) {
	value, ok := cs.Info.Pac.HubCatalogs.Load(catalogName)
	if !ok {
		return nil, fmt.Errorf("could not get details for catalog name: %s", catalogName)
	}
	catalogValue, ok := value.(settings.HubCatalog)
	if !ok {
		return nil, fmt.Errorf("could not get details for catalog name: %s", catalogName)
	}

	switch catalogValue.Type {
	case hubtypes.TektonHubType:
		return newTektonHubClient(cs, catalogValue.URL, catalogValue.Name), nil
	default:
		// defaulting to Artifact Hub
		return newArtifactHubClient(cs, catalogValue.URL, catalogValue.Name), nil
	}
}
