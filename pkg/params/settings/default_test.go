package settings

import (
	"net/http"
	"sync"
	"testing"

	hubtypes "github.com/openshift-pipelines/pipelines-as-code/pkg/hub/vars"
	testhttp "github.com/openshift-pipelines/pipelines-as-code/pkg/test/http"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func TestGetCatalogHub(t *testing.T) {
	hubCatalog := sync.Map{}
	hubCatalog.Store("custom", HubCatalog{
		Index: "1",
		URL:   "https://foo.com",
		Name:  "tekton",
	})

	// URLs for mocked HTTP responses
	const (
		artifactHubURL = "https://artifacthub.example.com"
		tektonHubURL   = "https://tektonhub.example.com"
	)

	// Mock HTTP client for API-based type detection
	mockHTTPClient := testhttp.MakeHTTPTestClient(map[string]map[string]string{
		artifactHubURL + "/api/v1/stats": {
			"code": "200",
		},
		tektonHubURL + "/api/v1/stats": {
			"code": "404",
		},
	})

	tests := []struct {
		name           string
		config         map[string]string
		numCatalogs    int
		wantLog        string
		hubCatalogs    *sync.Map
		wantCustomType map[string]string
		httpClient     *http.Client
	}{
		{
			name:        "good/default catalog",
			numCatalogs: 1,
			hubCatalogs: &sync.Map{},
			httpClient:  mockHTTPClient,
		},
		{
			name: "good/custom catalog",
			config: map[string]string{
				"catalog-1-id":   "custom",
				"catalog-1-url":  "https://foo.com",
				"catalog-1-name": "tekton",
			},
			numCatalogs: 2,
			hubCatalogs: &sync.Map{},
			wantLog:     "CONFIG: setting custom hub custom, catalog https://foo.com",
			httpClient:  mockHTTPClient,
		},
		{
			name: "good/custom catalog with same data",
			config: map[string]string{
				"catalog-1-id":   "custom",
				"catalog-1-url":  "https://foo.com",
				"catalog-1-name": "tekton",
			},
			numCatalogs: 2,
			hubCatalogs: &hubCatalog,
			wantLog:     "",
			httpClient:  mockHTTPClient,
		},
		{
			name: "good/custom catalog with different data",
			config: map[string]string{
				"catalog-1-id":   "custom",
				"catalog-1-url":  "https://bar.com",
				"catalog-1-name": "tekton",
			},
			numCatalogs: 2,
			hubCatalogs: &hubCatalog,
			wantLog:     "CONFIG: setting custom hub custom, catalog https://bar.com",
			httpClient:  mockHTTPClient,
		},
		{
			name: "good/custom catalog with initialization",
			config: map[string]string{
				"catalog-1-id":   "custom",
				"catalog-1-url":  "https://foo.com",
				"catalog-1-name": "tekton",
			},
			numCatalogs: 2,
			hubCatalogs: nil,
			wantLog:     "CONFIG: setting custom hub custom, catalog https://foo.com",
			httpClient:  mockHTTPClient,
		},
		{
			name: "bad/missing keys custom catalog",
			config: map[string]string{
				"catalog-1-id":   "custom",
				"catalog-1-name": "tekton",
			},
			numCatalogs: 1,
			hubCatalogs: &sync.Map{},
			wantLog:     "CONFIG: hub 1 should have the key catalog-1-url, skipping catalog configuration",
			httpClient:  mockHTTPClient,
		},
		{
			name: "bad/missing value for custom catalog",
			config: map[string]string{
				"catalog-1-id":   "custom",
				"catalog-1-name": "tekton",
				"catalog-1-url":  "",
			},
			numCatalogs: 1,
			hubCatalogs: &sync.Map{},
			wantLog:     "CONFIG: hub 1 catalog configuration have empty value for key catalog-1-url, skipping catalog configuration",
			httpClient:  mockHTTPClient,
		},
		{
			name: "bad/custom catalog called https",
			config: map[string]string{
				"catalog-1-id":   "https",
				"catalog-1-url":  "https://foo.com",
				"catalog-1-name": "tekton",
			},
			numCatalogs: 1,
			hubCatalogs: &sync.Map{},
			wantLog:     "CONFIG: custom hub catalog name cannot be https, skipping catalog configuration",
			httpClient:  mockHTTPClient,
		},
		{
			name: "bad/invalid url",
			config: map[string]string{
				"catalog-1-id":   "custom",
				"catalog-1-url":  "/u1!@1!@#$afoo.com",
				"catalog-1-name": "tekton",
			},
			numCatalogs: 1,
			hubCatalogs: &sync.Map{},
			wantLog:     "catalog url /u1!@1!@#$afoo.com is not valid, skipping catalog configuration",
			httpClient:  mockHTTPClient,
		},
		{
			name: "multiple catalogs with different types",
			config: map[string]string{
				"catalog-1-id":   "tektonhub",
				"catalog-1-url":  "https://tektonhub.com",
				"catalog-1-name": "tekton",
				"catalog-1-type": "tektonhub",
				"catalog-2-id":   "artifact",
				"catalog-2-url":  "https://artifacthub.com",
				"catalog-2-name": "artifacthub",
				"catalog-2-type": "artifacthub",
			},
			numCatalogs: 3, // default + 2 custom
			hubCatalogs: &sync.Map{},
			wantLog:     "CONFIG: setting custom hub tektonhub, catalog https://tektonhub.com",
			httpClient:  mockHTTPClient,
		},
		{
			name: "invalid hub type",
			config: map[string]string{
				"hub-catalog-type": "invalid",
			},
			numCatalogs: 1,
			hubCatalogs: &sync.Map{},
			wantLog:     `CONFIG: invalid hub type invalid, defaulting to artifacthub`,
			httpClient:  mockHTTPClient,
		},
		{
			name: "custom catalog type detection via API - success (ArtifactHub)",
			config: map[string]string{
				"catalog-1-id":   "example-ah",
				"catalog-1-url":  artifactHubURL,
				"catalog-1-name": "artifact",
			},
			numCatalogs: 2,
			hubCatalogs: &sync.Map{},
			wantCustomType: map[string]string{
				"example-ah": hubtypes.ArtifactHubType,
			},
			httpClient: mockHTTPClient,
		},
		{
			name: "custom catalog type detection via API - failure (TektonHub)",
			config: map[string]string{
				"catalog-1-id":   "example-th",
				"catalog-1-url":  tektonHubURL,
				"catalog-1-name": "tekton",
			},
			numCatalogs: 2,
			hubCatalogs: &sync.Map{},
			wantCustomType: map[string]string{
				"example-th": hubtypes.TektonHubType,
			},
			httpClient: mockHTTPClient,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			observer, catcher := zapobserver.New(zap.InfoLevel)
			fakelogger := zap.New(observer).Sugar()
			if tt.config == nil {
				tt.config = map[string]string{}
			}
			catalogs := getHubCatalogs(fakelogger, tt.hubCatalogs, tt.config, tt.httpClient)
			length := 0
			catalogs.Range(func(_, _ any) bool {
				length++
				return true
			})
			assert.Equal(t, length, tt.numCatalogs)
			if tt.wantLog != "" {
				assert.Assert(t, len(catcher.FilterMessageSnippet(tt.wantLog).TakeAll()) > 0, "could not find log message: got ", catcher)
			}
			for catalogID, expectedType := range tt.wantCustomType {
				value, ok := catalogs.Load(catalogID)
				assert.Assert(t, ok, "catalog %s should exist", catalogID)
				catalog, ok := value.(HubCatalog)
				assert.Assert(t, ok, "catalog %s should be HubCatalog type", catalogID)
				assert.Equal(t, catalog.Type, expectedType)
			}
			cmp.Equal(catalogs, tt.hubCatalogs)
		})
	}
}

func TestGetHubCatalogTypeViaAPI(t *testing.T) {
	tests := []struct {
		name           string
		serverStatus   string
		expectedResult string
	}{
		{
			name:           "returns ArtifactHubType on 200 OK",
			serverStatus:   "200",
			expectedResult: hubtypes.ArtifactHubType,
		},
		{
			name:           "returns TektonHubType on 404 Not Found",
			serverStatus:   "404",
			expectedResult: hubtypes.TektonHubType,
		},
		{
			name:           "returns TektonHubType on 500 Internal Server Error",
			serverStatus:   "500",
			expectedResult: hubtypes.TektonHubType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testURL := "https://test-hub.example.com"
			mockClient := testhttp.MakeHTTPTestClient(map[string]map[string]string{
				testURL + "/api/v1/stats": {
					"code": tt.serverStatus,
				},
			})

			result := getHubCatalogTypeViaAPI(testURL, mockClient)
			assert.Equal(t, result, tt.expectedResult)
		})
	}
}
