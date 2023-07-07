package settings

import (
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
}

func TestGetCatalogHub(t *testing.T) {
	config := make(map[string]string)
	config["catalog-1-id"] = "custom"
	config["catalog-1-url"] = "https://foo.com"
	config["catalog-1-name"] = "https://foo.com"
	tests := []struct {
		name             string
		config           map[string]string
		numCatalogs      int
		wantLog          string
		existingSettings *Settings
	}{
		{
			name:        "good/default catalog",
			numCatalogs: 1,
		},
		{
			name: "good/custom catalog",
			config: map[string]string{
				"catalog-1-id":   "custom",
				"catalog-1-url":  "https://foo.com",
				"catalog-1-name": "tekton",
			},
			numCatalogs: 2,
			wantLog:     "CONFIG: setting custom hub custom, catalog https://foo.com",
		},
		{
			name: "bad/missing keys custom catalog",
			config: map[string]string{
				"catalog-1-id":   "custom",
				"catalog-1-name": "tekton",
			},
			numCatalogs: 1,
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
			if tt.existingSettings == nil {
				tt.existingSettings = &Settings{}
			}
			catalogs := gethHubCatalogs(fakelogger, tt.existingSettings, tt.config)
			assert.Equal(t, len(catalogs), tt.numCatalogs)
			if tt.wantLog != "" {
				assert.Assert(t, len(catcher.FilterMessageSnippet(tt.wantLog).TakeAll()) > 0, "could not find log message: got ", catcher)
			}
		})
	}
}
