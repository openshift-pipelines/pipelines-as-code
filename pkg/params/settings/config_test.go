package settings

import (
	"testing"

	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
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
