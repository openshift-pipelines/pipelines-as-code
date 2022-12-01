package secrets

import (
	"strings"
	"testing"

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
	}{
		{
			name:                    "Target secret not there",
			targetNS:                nsNotThere,
			event:                   event,
			expectedGitCredentials:  "https://git:verysecrete@forge/owner/repo",
			expectedStartSecretName: "pac-gitauth-owner-repo",
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
