package settings

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  map[string]string
		wantErr string
	}{
		{
			name: "valid config",
			config: map[string]string{
				SecretAutoCreateKey:            "true",
				RemoteTasksKey:                 "false",
				BitbucketCloudCheckSourceIPKey: "false",
				MaxKeepRunUpperLimitKey:        "7",
				DefaultMaxKeepRunsKey:          "8",
				RememberOKToTestKey:            "true",
			},
			wantErr: "",
		},
		{
			name: "invalid bool",
			config: map[string]string{
				SecretAutoCreateKey: "random",
			},
			wantErr: "invalid value for key secret-auto-create, acceptable values: true or false",
		},
		{
			name: "invalid bool",
			config: map[string]string{
				RememberOKToTestKey: "random",
			},
			wantErr: "invalid value for key remember-ok-to-test, acceptable values: true or false",
		},
		{
			name: "invalid max keep run upper limit",
			config: map[string]string{
				MaxKeepRunUpperLimitKey: "random",
			},
			wantErr: "failed to convert max-keep-run-upper-limit value to int: strconv.Atoi: parsing \"random\": invalid syntax",
		},
		{
			name: "invalid max keep run default",
			config: map[string]string{
				DefaultMaxKeepRunsKey: "1as",
			},
			wantErr: "failed to convert default-max-keep-runs value to int: strconv.Atoi: parsing \"1as\": invalid syntax",
		},
		{
			name: "invalid check source ip value",
			config: map[string]string{
				BitbucketCloudCheckSourceIPKey: "ncntru",
			},
			wantErr: "invalid value for key bitbucket-cloud-check-source-ip, acceptable values: true or false",
		},
		{
			name: "invalid url value",
			config: map[string]string{
				TektonDashboardURLKey: "abc.xyz",
			},
			wantErr: "invalid value for key tekton-dashboard-url, invalid url: parse \"abc.xyz\": invalid URI for request",
		},
		{
			name: "empty values",
			config: map[string]string{
				RemoteTasksKey:                 "",
				SecretAutoCreateKey:            "",
				BitbucketCloudCheckSourceIPKey: "",
				TektonDashboardURLKey:          "",
				MaxKeepRunUpperLimitKey:        "",
				AutoConfigureNewGitHubRepoKey:  "",
				DefaultMaxKeepRunsKey:          "",
			},
			wantErr: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.config)
			if tt.wantErr != "" {
				assert.Error(t, err, tt.wantErr)
				return
			}
			assert.NilError(t, err)
		})
	}
}
