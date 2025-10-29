package llm

import (
	"context"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/llm/ltypes"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	paramclients "github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	kitesthelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test/kubernetestint"
	"gotest.tools/v3/assert"
)

func TestFactory_ValidateConfig(t *testing.T) {
	run := &params.Run{}
	ktesthelper := &kitesthelper.KinterfaceTest{}
	factory := NewFactory(run, ktesthelper)

	tests := []struct {
		name      string
		config    *ClientConfig
		wantError bool
	}{
		{
			name: "valid openai config",
			config: &ClientConfig{
				Provider: ltypes.LLMProviderOpenAI,
				TokenSecretRef: &v1alpha1.Secret{
					Name: "test-secret",
					Key:  "token",
				},
				TimeoutSeconds: 30,
				MaxTokens:      1000,
			},
			wantError: false,
		},
		{
			name: "valid gemini config",
			config: &ClientConfig{
				Provider: ltypes.LLMProviderGemini,
				TokenSecretRef: &v1alpha1.Secret{
					Name: "test-secret",
					Key:  "api_key",
				},
				TimeoutSeconds: 45,
				MaxTokens:      2000,
			},
			wantError: false,
		},
		{
			name:      "nil config",
			config:    nil,
			wantError: true,
		},
		{
			name: "missing provider",
			config: &ClientConfig{
				TokenSecretRef: &v1alpha1.Secret{
					Name: "test-secret",
					Key:  "token",
				},
			},
			wantError: true,
		},
		{
			name: "missing token secret ref",
			config: &ClientConfig{
				Provider: ltypes.LLMProviderOpenAI,
			},
			wantError: true,
		},
		{
			name: "invalid provider",
			config: &ClientConfig{
				Provider: "invalid-provider",
				TokenSecretRef: &v1alpha1.Secret{
					Name: "test-secret",
					Key:  "token",
				},
			},
			wantError: true,
		},
		{
			name: "negative timeout",
			config: &ClientConfig{
				Provider: ltypes.LLMProviderOpenAI,
				TokenSecretRef: &v1alpha1.Secret{
					Name: "test-secret",
					Key:  "token",
				},
				TimeoutSeconds: -1,
			},
			wantError: true,
		},
		{
			name: "negative max tokens",
			config: &ClientConfig{
				Provider: ltypes.LLMProviderOpenAI,
				TokenSecretRef: &v1alpha1.Secret{
					Name: "test-secret",
					Key:  "token",
				},
				MaxTokens: -1,
			},
			wantError: true,
		},
		{
			name: "valid config with custom api_url",
			config: &ClientConfig{
				Provider: ltypes.LLMProviderOpenAI,
				APIURL:   "https://custom-openai.example.com/v1",
				TokenSecretRef: &v1alpha1.Secret{
					Name: "test-secret",
					Key:  "token",
				},
			},
			wantError: false,
		},
		{
			name: "valid config with http api_url",
			config: &ClientConfig{
				Provider: ltypes.LLMProviderGemini,
				APIURL:   "http://localhost:8080/v1",
				TokenSecretRef: &v1alpha1.Secret{
					Name: "test-secret",
					Key:  "token",
				},
			},
			wantError: false,
		},
		{
			name: "invalid api_url - wrong scheme",
			config: &ClientConfig{
				Provider: ltypes.LLMProviderOpenAI,
				APIURL:   "ftp://example.com",
				TokenSecretRef: &v1alpha1.Secret{
					Name: "test-secret",
					Key:  "token",
				},
			},
			wantError: true,
		},
		{
			name: "invalid api_url - missing scheme",
			config: &ClientConfig{
				Provider: ltypes.LLMProviderOpenAI,
				APIURL:   "example.com/v1",
				TokenSecretRef: &v1alpha1.Secret{
					Name: "test-secret",
					Key:  "token",
				},
			},
			wantError: true,
		},
		{
			name: "invalid api_url - malformed",
			config: &ClientConfig{
				Provider: ltypes.LLMProviderOpenAI,
				APIURL:   "://invalid",
				TokenSecretRef: &v1alpha1.Secret{
					Name: "test-secret",
					Key:  "token",
				},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := factory.ValidateConfig(tt.config)
			if tt.wantError {
				assert.Assert(t, err != nil, "expected error but got none")
			} else {
				assert.NilError(t, err)
			}
		})
	}
}

func TestFactory_CreateClient(t *testing.T) {
	tests := []struct {
		name         string
		config       *ClientConfig
		namespace    string
		wantError    bool
		secretResult map[string]string
	}{
		{
			name: "create openai client",
			secretResult: map[string]string{
				"test-secret": "test-api",
			},
			config: &ClientConfig{
				Provider: ltypes.LLMProviderOpenAI,
				TokenSecretRef: &v1alpha1.Secret{
					Name: "test-secret",
					Key:  "token",
				},
				TimeoutSeconds: 30,
				MaxTokens:      1000,
			},
			namespace: "default",
			wantError: false,
		},
		{
			name: "create gemini client",
			secretResult: map[string]string{
				"test-secret": "test-api",
			},
			config: &ClientConfig{
				Provider: ltypes.LLMProviderGemini,
				TokenSecretRef: &v1alpha1.Secret{
					Name: "test-secret",
					Key:  "token",
				},
				TimeoutSeconds: 30,
				MaxTokens:      1000,
			},
			namespace: "default",
			wantError: false,
		},
		{
			name: "missing secret",
			config: &ClientConfig{
				Provider: ltypes.LLMProviderOpenAI,
				TokenSecretRef: &v1alpha1.Secret{
					Name: "missing-secret",
					Key:  "token",
				},
			},
			namespace: "default",
			wantError: true,
		},
		{
			name: "unsupported provider",
			config: &ClientConfig{
				Provider: "unsupported",
				TokenSecretRef: &v1alpha1.Secret{
					Name: "test-secret",
					Key:  "token",
				},
			},
			namespace: "default",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			// Create fake Kubernetes client with secret
			run := &params.Run{
				Clients: paramclients.Clients{},
			}
			ktesthelper := &kitesthelper.KinterfaceTest{
				GetSecretResult: tt.secretResult,
			}
			factory := NewFactory(run, ktesthelper)
			client, err := factory.CreateClient(ctx, tt.config, tt.namespace)

			if tt.wantError {
				assert.Assert(t, err != nil, "expected error but got none")
				assert.Assert(t, client == nil, "expected nil client on error")
			} else {
				assert.NilError(t, err)
				assert.Assert(t, client != nil, "expected non-nil client")

				// Verify client type matches provider
				switch tt.config.Provider {
				case ltypes.LLMProviderOpenAI:
					assert.Equal(t, client.GetProviderName(), string(ltypes.LLMProviderOpenAI))
				case ltypes.LLMProviderGemini:
					assert.Equal(t, client.GetProviderName(), string(ltypes.LLMProviderGemini))
				}
			}
		})
	}
}

func TestFactory_GetSupportedProviders(t *testing.T) {
	run := &params.Run{}

	ktesthelper := &kitesthelper.KinterfaceTest{}
	factory := NewFactory(run, ktesthelper)

	providers := factory.GetSupportedProviders()

	assert.Assert(t, len(providers) >= 2, "expected at least 2 supported providers")

	// Check that OpenAI and Gemini are supported
	var hasOpenAI, hasGemini bool
	for _, provider := range providers {
		switch provider {
		case ltypes.LLMProviderOpenAI:
			hasOpenAI = true
		case ltypes.LLMProviderGemini:
			hasGemini = true
		}
	}

	assert.Assert(t, hasOpenAI, "expected OpenAI to be supported")
	assert.Assert(t, hasGemini, "expected Gemini to be supported")
}

func TestFactory_CreateClientFromProvider(t *testing.T) {
	run := &params.Run{Clients: paramclients.Clients{}}
	ktesthelper := &kitesthelper.KinterfaceTest{
		GetSecretResult: map[string]string{
			"test-secret": "test-api-key",
		},
	}
	factory := NewFactory(run, ktesthelper)

	ctx := context.Background()
	client, err := factory.CreateClientFromProvider(
		ctx,
		"openai",
		"test-secret",
		"token",
		"default",
		30,
		1000,
	)

	assert.NilError(t, err)
	assert.Assert(t, client != nil)
	assert.Equal(t, client.GetProviderName(), "openai")
}

func TestFactory_CreateClient_WithModel(t *testing.T) {
	run := &params.Run{}
	ktesthelper := &kitesthelper.KinterfaceTest{}
	factory := NewFactory(run, ktesthelper)

	tests := []struct {
		name      string
		config    *ClientConfig
		wantError bool
	}{
		{
			name: "openai with custom model",
			config: &ClientConfig{
				Provider: ltypes.LLMProviderOpenAI,
				Model:    "gpt-5",
				TokenSecretRef: &v1alpha1.Secret{
					Name: "test-secret",
					Key:  "token",
				},
				TimeoutSeconds: 30,
				MaxTokens:      1000,
			},
			wantError: false,
		},
		{
			name: "gemini with custom model",
			config: &ClientConfig{
				Provider: ltypes.LLMProviderGemini,
				Model:    "gemini-2.5-pro",
				TokenSecretRef: &v1alpha1.Secret{
					Name: "test-secret",
					Key:  "token",
				},
				TimeoutSeconds: 30,
				MaxTokens:      1000,
			},
			wantError: false,
		},
		{
			name: "openai with empty model uses default",
			config: &ClientConfig{
				Provider: ltypes.LLMProviderOpenAI,
				Model:    "",
				TokenSecretRef: &v1alpha1.Secret{
					Name: "test-secret",
					Key:  "token",
				},
				TimeoutSeconds: 30,
				MaxTokens:      1000,
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := factory.ValidateConfig(tt.config)
			if tt.wantError {
				assert.Assert(t, err != nil)
			} else {
				assert.NilError(t, err)
			}
		})
	}
}

func TestGetDefaultModel(t *testing.T) {
	tests := []struct {
		name     string
		provider ltypes.AIProvider
		want     string
	}{
		{
			name:     "OpenAI default",
			provider: ltypes.LLMProviderOpenAI,
			want:     "gpt-5-mini",
		},
		{
			name:     "Gemini default",
			provider: ltypes.LLMProviderGemini,
			want:     "gemini-2.5-flash-lite",
		},
		{
			name:     "Unknown provider",
			provider: "unknown",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getDefaultModel(tt.provider)
			assert.Equal(t, got, tt.want)
		})
	}
}
