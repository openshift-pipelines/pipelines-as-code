package auth

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/h2non/gock"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"gotest.tools/v3/assert"
)

var (
	host  = "test.github.com"
	token = "gho_16C7e42F292c6912E7710c838347Ae178B4a"
)

func newIOStream() *cli.IOStreams {
	in := &bytes.Buffer{}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	return &cli.IOStreams{
		In:     io.NopCloser(in),
		Out:    out,
		ErrOut: errOut,
	}
}

func TestAuthFlow(t *testing.T) {
	baseURL := fmt.Sprintf("https://%s", host)

	defer gock.OffAll()

	ios := newIOStream()

	tests := []struct {
		name         string
		statusCode   int
		jsonResponse map[string]interface{}
		wantError    bool
		errorMsg     string
	}{
		{
			name:       "get verification code from github for authentication",
			statusCode: 200,
			jsonResponse: map[string]interface{}{
				"device_code":      "3584d83530557fdd1f46af8289938c8ef79f9dc5",
				"user_code":        "WDJB-MJHT",
				"verification_uri": "https://github.com/login/device",
				"expires_in":       900,
				"interval":         5,
			},
		},
		{
			name:       "error 500 unknown error",
			statusCode: 500,
			jsonResponse: map[string]interface{}{
				"error": "internal server error",
			},
			wantError: true,
			errorMsg:  "internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer gock.OffAll()

			gock.New(baseURL).
				Post("/login/device/code").
				Reply(tt.statusCode).
				JSON(tt.jsonResponse)

			gock.New(baseURL).
				Post("/login/oauth/access_token").
				Reply(200).
				JSON(map[string]interface{}{
					"access_token": token,
					"token_type":   "bearer",
					"scope":        "repo,gist",
				})

			got, err := RunAuthFlow(host, ios, "", []string{}, true, false)
			if tt.wantError {
				assert.Equal(t, err.Error(), tt.errorMsg)
			} else {
				assert.NilError(t, err)
				assert.Equal(t, token, got)
			}
		})
	}
}

func TestGetUserName(t *testing.T) {
	defer gock.Off()

	fakeToken := "gho_16C7e42F292c6912E7710c838347Ae178B4a"
	ios := newIOStream()
	userName := "zakisk"

	tests := []struct {
		name         string
		hostname     string
		statusCode   int
		jsonResponse map[string]interface{}
		wantError    bool
		errorMsg     string
	}{
		{
			name:       "get user name from github",
			hostname:   "https://api.github.com",
			statusCode: 200,
			jsonResponse: map[string]interface{}{
				"data": map[string]interface{}{
					"viewer": map[string]interface{}{
						"login": "zakisk",
					},
				},
			},
		},
		{
			name:       "wrong github host name",
			hostname:   "wronghost.com",
			statusCode: 200,
			wantError:  true,
			errorMsg:   "Post \"https://wronghost.com/api/graphql\": gock: cannot match any request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer gock.OffAll()

			gock.New(tt.hostname).
				Post("/graphql").
				MatchType("json").
				BodyString(`query UserCurrent\b`).
				Reply(tt.statusCode).
				JSON(tt.jsonResponse)

			got, err := GetViewer(tt.hostname, fakeToken, ios.ErrOut)
			if tt.wantError {
				assert.Equal(t, err.Error(), tt.errorMsg)
			} else {
				assert.NilError(t, err)
				assert.Equal(t, userName, got)
			}
		})
	}
}
