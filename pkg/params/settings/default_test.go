package settings

import (
	"sync"
	"testing"

	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
)

func TestSetDefaults(t *testing.T) {
	config := make(map[string]string)
	SetDefaults(config)
	assert.Equal(t, config[RemoteTasksKey], remoteTasksDefaultValue)
	assert.Equal(t, config[SecretAutoCreateKey], secretAutoCreateDefaultValue)
	assert.Equal(t, config[BitbucketCloudCheckSourceIPKey], bitbucketCloudCheckSourceIPDefaultValue)
	assert.Equal(t, config[ApplicationNameKey], PACApplicationNameDefaultValue)
	assert.Equal(t, config[RememberOKToTestKey], rememberOKToTestValue)
}

func TestGetCatalogHub(t *testing.T) {
	hubCatalog := sync.Map{}
	hubCatalog.Store("custom", HubCatalog{
		ID:   "custom",
		URL:  "https://foo.com",
		Name: "tekton",
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
			numCatalogs: 1,
			hubCatalogs: &sync.Map{},
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
			catalogs.Range(func(_, _ interface{}) bool {
				length++
				return true
			})
			assert.Equal(t, length, tt.numCatalogs)
			if tt.wantLog != "" {
				assert.Assert(t, len(catcher.FilterMessageSnippet(tt.wantLog).TakeAll()) > 0, "could not find log message: got ", catcher)
			}
		})
	}
}
