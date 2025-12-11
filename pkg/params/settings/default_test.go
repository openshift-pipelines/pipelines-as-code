package settings

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	hubtypes "github.com/openshift-pipelines/pipelines-as-code/pkg/hub/vars"
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
	tests := []struct {
		name        string
		config      map[string]string
		numCatalogs int
		wantLog     string
		hubCatalogs *sync.Map
	}{
		{
			name:        "good/default catalog",
			numCatalogs: 2,
			hubCatalogs: &sync.Map{},
		},
		{
			name: "good/custom catalog",
			config: map[string]string{
				"catalog-1-id":   "custom",
				"catalog-1-url":  "https://foo.com",
				"catalog-1-name": "tekton",
			},
			numCatalogs: 3,
			hubCatalogs: &sync.Map{},
			wantLog:     "CONFIG: setting custom hub custom, catalog https://foo.com",
		},
		{
			name: "good/custom catalog with same data",
			config: map[string]string{
				"catalog-1-id":   "custom",
				"catalog-1-url":  "https://foo.com",
				"catalog-1-name": "tekton",
			},
			numCatalogs: 3,
			hubCatalogs: &hubCatalog,
			wantLog:     "",
		},
		{
			name: "good/custom catalog with different data",
			config: map[string]string{
				"catalog-1-id":   "custom",
				"catalog-1-url":  "https://bar.com",
				"catalog-1-name": "tekton",
			},
			numCatalogs: 3,
			hubCatalogs: &hubCatalog,
			wantLog:     "CONFIG: setting custom hub custom, catalog https://bar.com",
		},
		{
			name: "good/custom catalog with initialization",
			config: map[string]string{
				"catalog-1-id":   "custom",
				"catalog-1-url":  "https://foo.com",
				"catalog-1-name": "tekton",
			},
			numCatalogs: 3,
			hubCatalogs: nil,
			wantLog:     "CONFIG: setting custom hub custom, catalog https://foo.com",
		},
		{
			name: "bad/missing keys custom catalog",
			config: map[string]string{
				"catalog-1-id":   "custom",
				"catalog-1-name": "tekton",
			},
			numCatalogs: 2,
			hubCatalogs: &sync.Map{},
			wantLog:     "CONFIG: hub 1 should have the key catalog-1-url, skipping catalog configuration",
		},
		{
			name: "bad/missing value for custom catalog",
			config: map[string]string{
				"catalog-1-id":   "custom",
				"catalog-1-name": "tekton",
				"catalog-1-url":  "",
			},
			numCatalogs: 2,
			hubCatalogs: &sync.Map{},
			wantLog:     "CONFIG: hub 1 catalog configuration have empty value for key catalog-1-url, skipping catalog configuration",
		},
		{
			name: "bad/custom catalog called https",
			config: map[string]string{
				"catalog-1-id":   "https",
				"catalog-1-url":  "https://foo.com",
				"catalog-1-name": "tekton",
			},
			numCatalogs: 2,
			hubCatalogs: &sync.Map{},
			wantLog:     "CONFIG: custom hub catalog name cannot be https, skipping catalog configuration",
		},
		{
			name: "bad/invalid url",
			config: map[string]string{
				"catalog-1-id":   "custom",
				"catalog-1-url":  "/u1!@1!@#$afoo.com",
				"catalog-1-name": "tekton",
			},
			numCatalogs: 2,
			hubCatalogs: &sync.Map{},
			wantLog:     "catalog url /u1!@1!@#$afoo.com is not valid, skipping catalog configuration",
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
		},
		{
			name: "invalid hub type",
			config: map[string]string{
				"hub-catalog-type": "invalid",
			},
			numCatalogs: 2,
			hubCatalogs: &sync.Map{},
			wantLog:     `CONFIG: invalid hub type invalid, defaulting to artifacthub`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			observer, catcher := zapobserver.New(zap.InfoLevel)
			fakelogger := zap.New(observer).Sugar()
			if tt.config == nil {
				tt.config = map[string]string{}
			}
			catalogs := getHubCatalogs(fakelogger, tt.hubCatalogs, tt.config)
			length := 0
			catalogs.Range(func(_, _ any) bool {
				length++
				return true
			})
			assert.Equal(t, length, tt.numCatalogs)
			if tt.wantLog != "" {
				assert.Assert(t, len(catcher.FilterMessageSnippet(tt.wantLog).TakeAll()) > 0, "could not find log message: got ", catcher)
			}
			cmp.Equal(catalogs, tt.hubCatalogs)
		})
	}
}

func TestGetHubCatalogTypeViaAPI(t *testing.T) {
	tests := []struct {
		name           string
		serverStatus   int
		expectedResult string
	}{
		{
			name:           "returns ArtifactHubType on 200 OK",
			serverStatus:   http.StatusOK,
			expectedResult: hubtypes.ArtifactHubType,
		},
		{
			name:           "returns TektonHubType on 404 Not Found",
			serverStatus:   http.StatusNotFound,
			expectedResult: hubtypes.TektonHubType,
		},
		{
			name:           "returns TektonHubType on 500 Internal Server Error",
			serverStatus:   http.StatusInternalServerError,
			expectedResult: hubtypes.TektonHubType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.serverStatus)
			}))
			defer server.Close()

			result := getHubCatalogTypeViaAPI(server.URL)
			assert.Equal(t, result, tt.expectedResult)
		})
	}
}
