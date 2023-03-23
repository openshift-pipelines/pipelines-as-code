package secrets

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"gotest.tools/v3/assert"
)

func TestCreateBasicAuthSecret(t *testing.T) {
	nsNotThere := "not_there"
	nsthere := "there"
	secrete := "verysecrete"

	event := info.Event{
		Organization: "owner",
		Repository:   "repo",
		URL:          "https://forge/owner/repo",
	}

	tests := []struct {
		name                    string
		targetNS                string
		event                   info.Event
		expectedGitCredentials  string
		expectedStartSecretName string
		expectedError           bool
		expectedLabels          map[string]string
	}{
		{
			name:                    "Target secret not there",
			targetNS:                nsNotThere,
			event:                   event,
			expectedGitCredentials:  "https://git:verysecrete@forge/owner/repo",
			expectedStartSecretName: "pac-gitauth-owner-repo",
			expectedLabels: map[string]string{
				"app.kubernetes.io/managed-by":              "pipelinesascode.tekton.dev",
				"pipelinesascode.tekton.dev/url-org":        "owner",
				"pipelinesascode.tekton.dev/url-repository": "repo",
			},
		},
		{
			name:     "Cleaned up gitlab style long repo and organisation name",
			targetNS: nsthere,
			event: info.Event{
				Organization: "owner/foo/bar/linux/kernel",
				Repository:   "yoyo",
				URL:          "https://forge/owner/yoyo/foo/bar/linux/kernel",
			},
			expectedGitCredentials:  "https://git:verysecrete@forge/owner/yoyo/foo/bar/linux/kernel",
			expectedStartSecretName: "pac-gitauth-owner-repo",
			expectedLabels: map[string]string{
				"app.kubernetes.io/managed-by":              "pipelinesascode.tekton.dev",
				"pipelinesascode.tekton.dev/url-org":        "owner-foo-bar-linux-kernel",
				"pipelinesascode.tekton.dev/url-repository": "yoyo",
			},
		},
		{
			name:                    "Use clone URL",
			targetNS:                nsNotThere,
			event:                   event,
			expectedGitCredentials:  "https://git:verysecrete@forge/owner/repo",
			expectedStartSecretName: "pac-gitauth-owner-repo",
		},
		{
			name:                    "Target secret already there",
			targetNS:                nsthere,
			event:                   event,
			expectedGitCredentials:  "https://git:verysecrete@forge/owner/repo",
			expectedStartSecretName: "pac-gitauth-owner-repo",
		},
		{
			name:     "Lowercase secrets",
			targetNS: nsthere,
			event: info.Event{
				Organization: "UPPER",
				Repository:   "CASE",
				URL:          "https://forge/UPPER/CASE",
			},
			expectedGitCredentials:  "https://git:verysecrete@forge/UPPER/CASE",
			expectedStartSecretName: "pac-gitauth-upper-case",
		},
		{
			name:     "Use clone URL",
			targetNS: nsthere,
			event: info.Event{
				Organization: "hello",
				Repository:   "moto",
				URL:          "https://forge/hello/moto",
				CloneURL:     "https://forge/miss/robinson",
			},
			expectedGitCredentials:  "https://git:verysecrete@forge/miss/robinson",
			expectedStartSecretName: "pac-gitauth-upper-case",
		},
		{
			name:     "different git user",
			targetNS: nsthere,
			event: info.Event{
				Organization: "hello",
				Repository:   "moto",
				URL:          "https://forge/bat/cave",
				Provider: &info.Provider{
					User:  "superman",
					Token: "supersecrete",
				},
			},
			expectedGitCredentials:  "https://superman:supersecrete@forge/bat/cave",
			expectedStartSecretName: "pac-gitauth-upper-case",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.event.Provider == nil {
				tt.event.Provider = &info.Provider{
					Token: secrete,
				}
			}
			secret, err := MakeBasicAuthSecret(&tt.event, tt.expectedStartSecretName)
			assert.NilError(t, err)
			if len(tt.expectedLabels) > 0 {
				if d := cmp.Diff(secret.GetLabels(), tt.expectedLabels); d != "" {
					t.Fatalf("-got, +want: %v", d)
				}
			}
			assert.Assert(t, strings.HasPrefix(secret.GetName(), tt.expectedStartSecretName))
			assert.Equal(t, secret.StringData[".git-credentials"], tt.expectedGitCredentials)
		})
	}
}

func TestGetBasicAuthSecret(t *testing.T) {
	t1 := GenerateBasicAuthSecretName()
	t2 := GenerateBasicAuthSecretName()
	assert.Assert(t, t1 != t2)
}
