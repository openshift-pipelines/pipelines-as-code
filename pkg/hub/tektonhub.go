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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
)

// tektonHubClient is a client for the Tekton Hub.
type tektonHubClient struct {
	params *params.Run
	url    string
	name   string
}

// newTektonHubClient returns a new Tekton Hub client.
func newTektonHubClient(params *params.Run, url, name string) Client {
	return &tektonHubClient{params: params, url: url, name: name}
}

type resourceVersionDataResponseBody struct {
	// ID is the unique id of resource's version
	ID *uint `json:"id,omitempty"`
	// Version of resource
	Version *string `json:"version,omitempty"`
	// Display name of version
	DisplayName *string `json:"displayName,omitempty"`
	// Description of version
	Description *string `json:"description,omitempty"`
	// Minimum pipelines version the resource's version is compatible with
	MinPipelinesVersion *string `json:"minPipelinesVersion,omitempty"`
	// Raw URL of resource's yaml file of the version
	RawURL *string `json:"rawURL,omitempty"`
	// Web URL of resource's yaml file of the version
	WebURL *string `json:"webURL,omitempty"`
	// Timestamp when version was last updated
	UpdatedAt *string `json:"updatedAt,omitempty"`
}

type hubResourceResponseBody struct {
	// ID is the unique id of the resource
	ID *uint `json:"id,omitempty"`
	// Name of resource
	Name *string `json:"name,omitempty"`
	// Kind of resource
	Kind *string `json:"kind,omitempty"`
	// Latest version of resource
	LatestVersion *resourceVersionDataResponseBody `json:"latestVersion,omitempty"`
	// List of all versions of a resource
	Versions []*resourceVersionDataResponseBody `json:"versions,omitempty"`
}

type hubResource struct {
	Data *hubResourceResponseBody `json:"data,omitempty"`
}

type hubResourceVersion struct {
	Data *resourceVersionDataResponseBody `json:"data,omitempty"`
}

// GetResource gets a resource from the Tekton Hub.
func (t *tektonHubClient) GetResource(ctx context.Context, _, resource, kind string) (string, error) {
	var rawURL string
	var err error

	if strings.Contains(resource, ":") {
		rawURL, err = t.getSpecificVersion(ctx, t.name, resource, kind)
	} else {
		rawURL, err = t.getLatestVersion(ctx, t.name, resource, kind)
	}
	if err != nil {
		return "", fmt.Errorf("could not fetch remote %s %s, hub API returned: %w", kind, resource, err)
	}

	data, err := t.params.Clients.GetURL(ctx, rawURL)
	if err != nil {
		return "", fmt.Errorf("could not fetch remote %s %s, hub API returned: %w", kind, resource, err)
	}
	return string(data), err
}

func (t *tektonHubClient) getSpecificVersion(ctx context.Context, catalogName, resource, kind string) (string, error) {
	split := strings.Split(resource, ":")
	version := split[len(split)-1]
	resourceName := split[0]
	url := fmt.Sprintf("%s/resource/%s/%s/%s/%s", t.url, catalogName, kind, resourceName, version)
	hr := hubResourceVersion{}
	data, err := t.params.Clients.GetURL(ctx, url)
	if err != nil {
		return "", fmt.Errorf("could not fetch specific %s version from the hub %s:%s: %w", kind, resource, version, err)
	}
	err = json.Unmarshal(data, &hr)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/raw", url), nil
}

func (t *tektonHubClient) getLatestVersion(ctx context.Context, catalogName, resource, kind string) (string, error) {
	url := fmt.Sprintf("%s/resource/%s/%s/%s", t.url, catalogName, kind, resource)
	hr := new(hubResource)
	data, err := t.params.Clients.GetURL(ctx, url)
	if err != nil {
		return "", err
	}
	err = json.Unmarshal(data, &hr)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s/%s/raw", url, *hr.Data.LatestVersion.Version), nil
}
