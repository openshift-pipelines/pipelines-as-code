package llm

import (
	"context"
	"fmt"
	"net/url"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	llmtypes "github.com/openshift-pipelines/pipelines-as-code/pkg/llm/ltypes"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/llm/providers/gemini"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/llm/providers/openai"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/secrets/types"
)

// ClientConfig holds the configuration needed to create LLM clients.
type ClientConfig struct {
	Provider       llmtypes.AIProvider
	APIURL         string
	Model          string // Model name to use (empty string uses provider default)
	TokenSecretRef *v1alpha1.Secret
	TimeoutSeconds int
	MaxTokens      int
}

// Factory creates LLM clients based on provider configuration.
type Factory struct {
	run       *params.Run
	kinteract kubeinteraction.Interface
}

// NewFactory creates a new LLM client factory.
func NewFactory(run *params.Run, kinteract kubeinteraction.Interface) *Factory {
	return &Factory{
		run:       run,
		kinteract: kinteract,
	}
}

// CreateClient creates an LLM client based on the provided configuration.
func (f *Factory) CreateClient(ctx context.Context, config *ClientConfig, namespace string) (llmtypes.Client, error) {
	if config == nil {
		return nil, fmt.Errorf("client configuration is required")
	}

	if err := f.ValidateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid client configuration: %w", err)
	}

	token, err := f.getTokenFromSecret(ctx, config.TokenSecretRef, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve LLM token: %w", err)
	}

	timeoutSeconds, maxTokens := f.applyDefaults(config.TimeoutSeconds, config.MaxTokens)
	baseURL := config.APIURL
	model := config.Model
	if model == "" {
		model = getDefaultModel(config.Provider)
	}

	baseClient, err := f.createProviderClient(config.Provider, token, baseURL, model, timeoutSeconds, maxTokens)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s client: %w", config.Provider, err)
	}

	return baseClient, nil
}

// ValidateConfig validates the client configuration.
func (f *Factory) ValidateConfig(config *ClientConfig) error {
	if config == nil {
		return fmt.Errorf("client configuration is required")
	}

	if config.Provider == "" {
		return fmt.Errorf("LLM provider is required")
	}

	if config.TokenSecretRef == nil {
		return fmt.Errorf("token secret reference is required")
	}
	if config.TokenSecretRef.Name == "" {
		return fmt.Errorf("token secret name is required")
	}

	if config.APIURL != "" {
		if err := f.validateURL(config.APIURL); err != nil {
			return fmt.Errorf("invalid api_url: %w", err)
		}
	}

	if !f.isProviderSupported(config.Provider) {
		return fmt.Errorf("unsupported LLM provider: %s", config.Provider)
	}

	if config.TimeoutSeconds < 0 {
		return fmt.Errorf("timeout seconds must be non-negative")
	}

	if config.MaxTokens < 0 {
		return fmt.Errorf("max tokens must be non-negative")
	}

	return nil
}

// GetSupportedProviders returns a list of supported LLM providers.
func (f *Factory) GetSupportedProviders() []llmtypes.AIProvider {
	return []llmtypes.AIProvider{
		llmtypes.LLMProviderOpenAI,
		llmtypes.LLMProviderGemini,
	}
}

// isProviderSupported checks if the given provider is supported.
func (f *Factory) isProviderSupported(provider llmtypes.AIProvider) bool {
	for _, supported := range f.GetSupportedProviders() {
		if provider == supported {
			return true
		}
	}
	return false
}

// validateURL validates that the URL is properly formatted.
func (f *Factory) validateURL(urlStr string) error {
	if urlStr == "" {
		return nil // Empty is valid (optional)
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("failed to parse URL '%s': %w", urlStr, err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("URL scheme must be 'http' or 'https', got '%s'", parsedURL.Scheme)
	}

	if parsedURL.Host == "" {
		return fmt.Errorf("URL must contain a host")
	}

	return nil
}

// getTokenFromSecret retrieves the API token from a Kubernetes secret.
func (f *Factory) getTokenFromSecret(ctx context.Context, secretRef *v1alpha1.Secret, namespace string) (string, error) {
	if secretRef == nil {
		return "", fmt.Errorf("secret reference is nil")
	}

	key := secretRef.Key
	if key == "" {
		key = "token"
	}

	opt := types.GetSecretOpt{
		Namespace: namespace,
		Name:      secretRef.Name,
		Key:       key,
	}

	secretValue, err := f.kinteract.GetSecret(ctx, opt)
	if err != nil {
		return "", fmt.Errorf("failed to get secret %s/%s: %w", namespace, secretRef.Name, err)
	}

	if secretValue == "" {
		return "", fmt.Errorf("secret %s/%s key %s is empty", namespace, secretRef.Name, key)
	}

	return secretValue, nil
}

// applyDefaults applies default values for timeout and max tokens if not specified.
func (f *Factory) applyDefaults(timeoutSeconds, maxTokens int) (int, int) {
	if timeoutSeconds == 0 {
		timeoutSeconds = llmtypes.DefaultConfig.TimeoutSeconds
	}
	if maxTokens == 0 {
		maxTokens = llmtypes.DefaultConfig.MaxTokens
	}
	return timeoutSeconds, maxTokens
}

// createProviderClient creates a provider-specific client.
func (f *Factory) createProviderClient(provider llmtypes.AIProvider, token, baseURL, model string, timeoutSeconds, maxTokens int) (llmtypes.Client, error) {
	switch provider {
	case llmtypes.LLMProviderOpenAI:
		config := &openai.Config{
			APIKey:         token,
			Model:          model,
			TimeoutSeconds: timeoutSeconds,
			MaxTokens:      maxTokens,
		}
		if baseURL != "" {
			config.BaseURL = baseURL
		}
		return openai.NewClient(config)
	case llmtypes.LLMProviderGemini:
		config := &gemini.Config{
			APIKey:         token,
			Model:          model,
			TimeoutSeconds: timeoutSeconds,
			MaxTokens:      maxTokens,
		}
		if baseURL != "" {
			config.BaseURL = baseURL
		}
		return gemini.NewClient(config)
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", provider)
	}
}

// CreateClientFromProvider creates a client directly from provider string and secret info.
func (f *Factory) CreateClientFromProvider(ctx context.Context, provider, secretName, secretKey, namespace string, timeoutSeconds, maxTokens int) (llmtypes.Client, error) {
	config := &ClientConfig{
		Provider: llmtypes.AIProvider(provider),
		APIURL:   "", // Use provider default
		TokenSecretRef: &v1alpha1.Secret{
			Name: secretName,
			Key:  secretKey,
		},
		TimeoutSeconds: timeoutSeconds,
		MaxTokens:      maxTokens,
	}

	if err := f.ValidateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid client configuration: %w", err)
	}

	return f.CreateClient(ctx, config, namespace)
}

// getDefaultModel returns the default model for a provider.
func getDefaultModel(provider llmtypes.AIProvider) string {
	switch provider {
	case llmtypes.LLMProviderOpenAI:
		return "gpt-5-mini"
	case llmtypes.LLMProviderGemini:
		return "gemini-2.5-flash-lite"
	default:
		return ""
	}
}
