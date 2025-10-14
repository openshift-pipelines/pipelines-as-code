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
				TokenSecretRef: &v1alpha1.LLMSecret{Secret: &v1alpha1.Secret{
					Name: "test-secret",
					Key:  "token",
				}},
				TimeoutSeconds: 30,
				MaxTokens:      1000,
			},
			wantError: false,
		},
		{
			name: "valid gemini config",
			config: &ClientConfig{
				Provider: ltypes.LLMProviderGemini,
				TokenSecretRef: &v1alpha1.LLMSecret{Secret: &v1alpha1.Secret{
					Name: "test-secret",
					Key:  "api_key",
				}},
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
				TokenSecretRef: &v1alpha1.LLMSecret{Secret: &v1alpha1.Secret{
					Name: "test-secret",
					Key:  "token",
				}},
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
				TokenSecretRef: &v1alpha1.LLMSecret{Secret: &v1alpha1.Secret{
					Name: "test-secret",
					Key:  "token",
				}},
			},
			wantError: true,
		},
		{
			name: "negative timeout",
			config: &ClientConfig{
				Provider: ltypes.LLMProviderOpenAI,
				TokenSecretRef: &v1alpha1.LLMSecret{Secret: &v1alpha1.Secret{
					Name: "test-secret",
					Key:  "token",
				}},
				TimeoutSeconds: -1,
			},
			wantError: true,
		},
		{
			name: "negative max tokens",
			config: &ClientConfig{
				Provider: ltypes.LLMProviderOpenAI,
				TokenSecretRef: &v1alpha1.LLMSecret{Secret: &v1alpha1.Secret{
					Name: "test-secret",
					Key:  "token",
				}},
				MaxTokens: -1,
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
				TokenSecretRef: &v1alpha1.LLMSecret{Secret: &v1alpha1.Secret{
					Name: "test-secret",
					Key:  "token",
				}},
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
				TokenSecretRef: &v1alpha1.LLMSecret{Secret: &v1alpha1.Secret{
					Name: "test-secret",
					Key:  "token",
				}},
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
				TokenSecretRef: &v1alpha1.LLMSecret{Secret: &v1alpha1.Secret{
					Name: "missing-secret",
					Key:  "token",
				}},
			},
			namespace: "default",
			wantError: true,
		},
		{
			name: "unsupported provider",
			config: &ClientConfig{
				Provider: "unsupported",
				TokenSecretRef: &v1alpha1.LLMSecret{Secret: &v1alpha1.Secret{
					Name: "test-secret",
					Key:  "token",
				}},
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
