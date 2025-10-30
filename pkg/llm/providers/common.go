// Package providers contains shared functionality for LLM provider clients.
package providers

import (
	"fmt"
	"net/url"
	"strings"
)

// CommonConfig represents the common configuration fields across all LLM providers.
type CommonConfig struct {
	APIKey         string
	TimeoutSeconds int
	MaxTokens      int
}

// ValidateCommonConfig validates common configuration fields.
// It checks that the API key is provided and that numeric fields are non-negative.
func ValidateCommonConfig(config any) error {
	if config == nil {
		return fmt.Errorf("config is required")
	}

	commonCfg, ok := config.(*CommonConfig)
	if !ok {
		return fmt.Errorf("invalid config type: expected *CommonConfig, got %T", config)
	}

	if commonCfg.APIKey == "" {
		return fmt.Errorf("API key is required")
	}

	if commonCfg.TimeoutSeconds < 0 {
		return fmt.Errorf("timeout seconds must be non-negative")
	}

	if commonCfg.MaxTokens < 0 {
		return fmt.Errorf("max tokens must be non-negative")
	}

	return nil
}

// ApplyDefaults applies default values to common configuration fields.
// It sets TimeoutSeconds to 30 if it's zero, and MaxTokens to 1000 if it's zero.
func ApplyDefaults(config any) error {
	if config == nil {
		return fmt.Errorf("config is required")
	}

	commonCfg, ok := config.(*CommonConfig)
	if !ok {
		return fmt.Errorf("invalid config type: expected *CommonConfig, got %T", config)
	}

	if commonCfg.TimeoutSeconds == 0 {
		commonCfg.TimeoutSeconds = 30
	}

	if commonCfg.MaxTokens == 0 {
		commonCfg.MaxTokens = 1000
	}

	return nil
}

// ValidateBaseURL validates that the provided baseURL is a valid HTTP/HTTPS URL.
func ValidateBaseURL(baseURL string) error {
	if baseURL == "" {
		return fmt.Errorf("base URL is required")
	}

	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("invalid base URL: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("base URL must use http or https scheme, got: %s", parsedURL.Scheme)
	}

	if parsedURL.Host == "" {
		return fmt.Errorf("base URL must have a valid host")
	}

	// Check for invalid characters that might cause issues
	if strings.ContainsAny(baseURL, " \t\n\r") {
		return fmt.Errorf("base URL contains invalid whitespace characters")
	}

	return nil
}
