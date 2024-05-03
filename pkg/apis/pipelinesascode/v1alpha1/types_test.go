package v1alpha1

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestMergeSpecs(t *testing.T) {
	two := 2
	incomings := &[]Incoming{{
		Type: "type",
		Secret: Secret{
			Name: "name",
		},
		Params: []string{"param1", "param2"},
	}}
	gp := &GitProvider{
		URL:  "url",
		User: "user",
		Secret: &Secret{
			Name: "name",
		},
		WebhookSecret: &Secret{
			Name: "webhook",
		},
		Type: "type1",
	}
	params := &[]Params{{Name: "name", Value: "value"}}
	tests := []struct {
		name     string
		local    *RepositorySpec
		global   RepositorySpec
		expected *RepositorySpec
	}{
		{
			name:  "global settings just params and concurrency",
			local: &RepositorySpec{},
			global: RepositorySpec{
				Params:           params,
				ConcurrencyLimit: &two,
			},
			expected: &RepositorySpec{
				Params:           params,
				ConcurrencyLimit: &two,
			},
		},
		{
			name: "global settings",
			local: &RepositorySpec{
				Settings:    &Settings{},
				GitProvider: &GitProvider{}, // Initialize as needed
			},
			global: RepositorySpec{
				Settings: &Settings{
					GithubAppTokenScopeRepos: []string{"repo1", "repo2"},
					PipelineRunProvenance:    "provenance",
					Policy: &Policy{
						OkToTest: []string{"ok1", "ok2"},
					},
				}, // Initialize as needed
				GitProvider:      gp, // Initialize as needed
				Incomings:        incomings,
				Params:           params,
				ConcurrencyLimit: &two,
			},
			expected: &RepositorySpec{
				Settings: &Settings{
					GithubAppTokenScopeRepos: []string{"repo1", "repo2"},
					PipelineRunProvenance:    "provenance",
					Policy: &Policy{
						OkToTest: []string{"ok1", "ok2"},
					},
				},
				Incomings:        incomings,
				GitProvider:      gp,
				Params:           params,
				ConcurrencyLimit: &two,
			},
		},
		{
			name: "local settings take precedence",
			local: &RepositorySpec{
				Settings: &Settings{
					GithubAppTokenScopeRepos: []string{"repo1", "repo2"},
					PipelineRunProvenance:    "provenance",
					Policy: &Policy{
						OkToTest: []string{"ok1", "ok2"},
					},
				}, // Initialize as needed
				GitProvider: &GitProvider{}, // Initialize as needed
			},
			global: RepositorySpec{
				Settings: &Settings{
					GithubAppTokenScopeRepos: []string{"hello", "moto"},
					PipelineRunProvenance:    "somewhere",
					Policy: &Policy{
						OkToTest: []string{"to", "be"},
					},
				}, // Initialize as needed
				GitProvider:      gp, // Initialize as needed
				Incomings:        incomings,
				Params:           params,
				ConcurrencyLimit: &two,
			},
			expected: &RepositorySpec{
				Settings: &Settings{
					GithubAppTokenScopeRepos: []string{"repo1", "repo2"},
					PipelineRunProvenance:    "provenance",
					Policy: &Policy{
						OkToTest: []string{"ok1", "ok2"},
					},
				},
				Incomings:        incomings,
				GitProvider:      gp,
				Params:           params,
				ConcurrencyLimit: &two,
			},
		},
		{
			name: "different git providers",
			local: &RepositorySpec{
				GitProvider: &GitProvider{
					Type: "local",
				}, // Initialize as needed
			},
			global: RepositorySpec{
				GitProvider: gp,
			},
			expected: &RepositorySpec{
				GitProvider: &GitProvider{
					Type: "local",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.local.Merge(tt.global)
			assert.DeepEqual(t, tt.expected, tt.local)
		})
	}
}
