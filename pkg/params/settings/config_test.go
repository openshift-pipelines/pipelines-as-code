package settings

import (
	"testing"

	"github.com/spf13/viper"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
)

func TestStringToBool(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "true",
			args: args{
				s: "true",
			},
			want: true,
		},
		{
			name: "false",
			args: args{
				s: "false",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StringToBool(tt.args.s); got != tt.want {
				t.Errorf("StringToBool() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfigToSettings(t *testing.T) {
	type args struct {
		setting *Settings
		config  map[string]string
	}
	tests := []struct {
		name            string
		args            args
		wantErr         bool
		wantLogContains string
	}{
		{
			name: "invalid",
			args: args{
				setting: &Settings{},
				config: map[string]string{
					SecretAutoCreateKey: "test",
				},
			},
			wantErr: true,
		},
		{
			name: "set application name",
			args: args{
				setting: &Settings{},
				config: map[string]string{
					ApplicationNameKey: "test",
				},
			},
			wantLogContains: "application name set to test",
		},
		{
			name: "set auto create key",
			args: args{
				setting: &Settings{},
				config: map[string]string{
					SecretAutoCreateKey: "true",
				},
			},
			wantLogContains: "secret auto create",
		},
		{
			name: "set remember-ok-to-test key",
			args: args{
				setting: &Settings{RememberOKToTest: true},
				config: map[string]string{
					RememberOKToTestKey: "false",
				},
			},
			wantLogContains: "remember ok-to-test",
		},
		{
			name: "set hub url",
			args: args{
				setting: &Settings{},
				config: map[string]string{
					HubURLKey: "https://test",
				},
			},
		},
		{
			name: "set hub name",
			args: args{
				setting: &Settings{},
				config: map[string]string{
					HubCatalogNameKey: "foo",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			observer, catchlog := zapobserver.New(zap.InfoLevel)
			logger := zap.New(observer).Sugar()
			if err := ConfigToSettings(logger, tt.args.setting, tt.args.config); (err != nil) != tt.wantErr {
				t.Errorf("ConfigToSettings() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantLogContains != "" {
				all := catchlog.FilterMessageSnippet(tt.wantLogContains).TakeAll()
				if len(all) == 0 {
					t.Errorf("ConfigToSettings() want log contains %v given: %v", tt.wantLogContains, all)
				}
			}
		})
	}
}

func TestReadHubCatalog(t *testing.T) {
	tests := []struct {
		name        string
		config      map[string]string
		numCatalogs int
		wantLog     string
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
			config := viper.New()
			if tt.config != nil {
				for k, v := range tt.config {
					config.Set(k, v)
				}
			}
			catalogs := readHubCatalog(config, fakelogger)
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
