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
				"PAC_CONTROLLER_LABEL":             "mylabel",
				"PAC_CONTROLLER_SECRET":            "mysecret",
				"PAC_CONTROLLER_CONFIGMAP":         "myconfigmap",
				"PAC_CONTROLLER_GLOBAL_REPOSITORY": "arepo",
			},
			want: &ControllerInfo{
				Name:             "mylabel",
				Configmap:        "myconfigmap",
				Secret:           "mysecret",
				GlobalRepository: "arepo",
			},
		},
		{
			name: "info from default",
			envs: map[string]string{},
			want: &ControllerInfo{
				Name:             defaultControllerLabel,
				Configmap:        DefaultPipelinesAscodeConfigmapName,
				Secret:           DefaultPipelinesAscodeSecretName,
				GlobalRepository: DefaultGlobalRepoName,
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
		{
			name: "Controller name present in context",
			args: args{
				ctx: context.WithValue(context.Background(), currentControllerName, "MyTestController"),
			},
			want: "MyTestController",
		},
		{
			name: "Controller name not present in context",
			args: args{
				ctx: context.Background(), // Empty context
			},
			want: "",
		},
		{
			name: "Value present but not a string",
			args: args{
				ctx: context.WithValue(context.Background(), currentControllerName, 12345), // Value is an int
			},
			want: "", // Expect empty string as per GetCurrentControllerName logic
		},
		{
			name: "Context with a different key (should not find ours)",
			args: args{
				ctx: context.WithValue(context.Background(), contextKey("otherKey"), "SomeOtherValue"),
			},
			want: "",
		},
		{
			name: "Context with our key and another key",
			args: args{
				ctx: context.WithValue(context.WithValue(context.Background(), contextKey("otherKey"), "SomeOtherValue"), currentControllerName, "PrimaryController"),
			},
			want: "PrimaryController",
		},
		{
			name: "Empty string as controller name",
			args: args{
				ctx: context.WithValue(context.Background(), currentControllerName, ""),
			},
			want: "", // If an empty string is stored, it should be retrieved as such.
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetCurrentControllerName(tt.args.ctx); got != tt.want {
				t.Errorf("GetCurrentControllerName() = %v, want %v", got, tt.want)
			}
		})
	}
}
