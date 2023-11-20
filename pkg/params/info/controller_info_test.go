package info

import (
	"context"
	"reflect"
	"testing"

	"gotest.tools/v3/assert"
)

func TestGetControllerInfoFromEnvOrDefault(t *testing.T) {
	tests := []struct {
		name string
		want *ControllerInfo
		envs map[string]string
	}{
		{
			name: "info with envs",
			envs: map[string]string{
				"PAC_CONTROLLER_LABEL":     "mylabel",
				"PAC_CONTROLLER_SECRET":    "mysecret",
				"PAC_CONTROLLER_CONFIGMAP": "myconfigmap",
			},
			want: &ControllerInfo{
				Name:      "mylabel",
				Configmap: "myconfigmap",
				Secret:    "mysecret",
			},
		},
		{
			name: "info from default",
			envs: map[string]string{},
			want: &ControllerInfo{
				Name:      defaultControllerLabel,
				Configmap: DefaultPipelinesAscodeConfigmapName,
				Secret:    DefaultPipelinesAscodeSecretName,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envs {
				t.Setenv(k, v)
			}
			if got := GetControllerInfoFromEnvOrDefault(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetControllerInfoFromEnvOrDefault() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetStoreCurrentControllerName(t *testing.T) {
	type args struct {
		name string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "store controller name",
			args: args{
				name: "mycontroller",
			},
			want: "mycontroller",
		},
		{
			name: "did not get any",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.args.name != "" {
				ctx = StoreCurrentControllerName(ctx, tt.args.name)
			}
			got := GetCurrentControllerName(ctx)
			assert.Equal(t, got, tt.want)
		})
	}
}

func TestGetCurrentControllerName(t *testing.T) {
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetCurrentControllerName(tt.args.ctx); got != tt.want {
				t.Errorf("GetCurrentControllerName() = %v, want %v", got, tt.want)
			}
		})
	}
}
