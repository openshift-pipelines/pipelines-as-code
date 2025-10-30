package providers

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestValidateCommonConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  any
		wantErr string
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: "config is required",
		},
		{
			name:    "wrong type",
			config:  "not a config",
			wantErr: "invalid config type: expected *CommonConfig, got string",
		},
		{
			name: "valid config",
			config: &CommonConfig{
				APIKey:         "test-key",
				TimeoutSeconds: 30,
				MaxTokens:      1000,
			},
			wantErr: "",
		},
		{
			name: "empty API key",
			config: &CommonConfig{
				APIKey:         "",
				TimeoutSeconds: 30,
				MaxTokens:      1000,
			},
			wantErr: "API key is required",
		},
		{
			name: "negative timeout seconds",
			config: &CommonConfig{
				APIKey:         "test-key",
				TimeoutSeconds: -1,
				MaxTokens:      1000,
			},
			wantErr: "timeout seconds must be non-negative",
		},
		{
			name: "negative max tokens",
			config: &CommonConfig{
				APIKey:         "test-key",
				TimeoutSeconds: 30,
				MaxTokens:      -1,
			},
			wantErr: "max tokens must be non-negative",
		},
		{
			name: "zero timeout seconds is valid",
			config: &CommonConfig{
				APIKey:         "test-key",
				TimeoutSeconds: 0,
				MaxTokens:      1000,
			},
			wantErr: "",
		},
		{
			name: "zero max tokens is valid",
			config: &CommonConfig{
				APIKey:         "test-key",
				TimeoutSeconds: 30,
				MaxTokens:      0,
			},
			wantErr: "",
		},
		{
			name: "all zero values except API key",
			config: &CommonConfig{
				APIKey:         "test-key",
				TimeoutSeconds: 0,
				MaxTokens:      0,
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCommonConfig(tt.config)
			if tt.wantErr == "" {
				assert.NilError(t, err)
			} else {
				assert.Error(t, err, tt.wantErr)
			}
		})
	}
}

func TestApplyDefaults(t *testing.T) {
	tests := []struct {
		name       string
		config     any
		wantErr    string
		wantConfig *CommonConfig
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: "config is required",
		},
		{
			name:    "wrong type",
			config:  "not a config",
			wantErr: "invalid config type: expected *CommonConfig, got string",
		},
		{
			name: "zero timeout and max tokens get defaults",
			config: &CommonConfig{
				APIKey:         "test-key",
				TimeoutSeconds: 0,
				MaxTokens:      0,
			},
			wantErr: "",
			wantConfig: &CommonConfig{
				APIKey:         "test-key",
				TimeoutSeconds: 30,
				MaxTokens:      1000,
			},
		},
		{
			name: "non-zero values are preserved",
			config: &CommonConfig{
				APIKey:         "test-key",
				TimeoutSeconds: 45,
				MaxTokens:      2000,
			},
			wantErr: "",
			wantConfig: &CommonConfig{
				APIKey:         "test-key",
				TimeoutSeconds: 45,
				MaxTokens:      2000,
			},
		},
		{
			name: "only timeout needs default",
			config: &CommonConfig{
				APIKey:         "test-key",
				TimeoutSeconds: 0,
				MaxTokens:      500,
			},
			wantErr: "",
			wantConfig: &CommonConfig{
				APIKey:         "test-key",
				TimeoutSeconds: 30,
				MaxTokens:      500,
			},
		},
		{
			name: "only max tokens needs default",
			config: &CommonConfig{
				APIKey:         "test-key",
				TimeoutSeconds: 60,
				MaxTokens:      0,
			},
			wantErr: "",
			wantConfig: &CommonConfig{
				APIKey:         "test-key",
				TimeoutSeconds: 60,
				MaxTokens:      1000,
			},
		},
		{
			name: "negative values are preserved (not corrected)",
			config: &CommonConfig{
				APIKey:         "test-key",
				TimeoutSeconds: -1,
				MaxTokens:      -100,
			},
			wantErr: "",
			wantConfig: &CommonConfig{
				APIKey:         "test-key",
				TimeoutSeconds: -1,
				MaxTokens:      -100,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ApplyDefaults(tt.config)
			if tt.wantErr == "" {
				assert.NilError(t, err)
				if tt.wantConfig != nil {
					assert.DeepEqual(t, tt.config, tt.wantConfig)
				}
			} else {
				assert.Error(t, err, tt.wantErr)
			}
		})
	}
}

func TestValidateAndApplyDefaults(t *testing.T) {
	// Test that we can apply defaults and then validate
	config := &CommonConfig{
		APIKey:         "test-key",
		TimeoutSeconds: 0,
		MaxTokens:      0,
	}

	// Apply defaults
	err := ApplyDefaults(config)
	assert.NilError(t, err)

	// Validate should pass
	err = ValidateCommonConfig(config)
	assert.NilError(t, err)

	// Check values were set correctly
	assert.Equal(t, config.TimeoutSeconds, 30)
	assert.Equal(t, config.MaxTokens, 1000)
}

func TestValidateBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		wantErr string
	}{
		{
			name:    "empty URL",
			baseURL: "",
			wantErr: "base URL is required",
		},
		{
			name:    "valid HTTPS URL",
			baseURL: "https://api.example.com",
			wantErr: "",
		},
		{
			name:    "valid HTTP URL",
			baseURL: "http://api.example.com",
			wantErr: "",
		},
		{
			name:    "valid URL with port",
			baseURL: "https://api.example.com:8443",
			wantErr: "",
		},
		{
			name:    "valid URL with path",
			baseURL: "https://api.example.com/v1",
			wantErr: "",
		},
		{
			name:    "invalid URL - no scheme",
			baseURL: "api.example.com",
			wantErr: "base URL must use http or https scheme",
		},
		{
			name:    "invalid URL - wrong scheme (ftp)",
			baseURL: "ftp://api.example.com",
			wantErr: "base URL must use http or https scheme",
		},
		{
			name:    "invalid URL - wrong scheme (ws)",
			baseURL: "ws://api.example.com",
			wantErr: "base URL must use http or https scheme",
		},
		{
			name:    "invalid URL - no host",
			baseURL: "https://",
			wantErr: "base URL must have a valid host",
		},
		{
			name:    "invalid URL - with whitespace",
			baseURL: "https://api.example.com /path",
			wantErr: "invalid base URL",
		},
		{
			name:    "invalid URL - with tab",
			baseURL: "https://api.example.com\t/path",
			wantErr: "invalid base URL",
		},
		{
			name:    "invalid URL - with newline",
			baseURL: "https://api.example.com\n",
			wantErr: "invalid base URL",
		},
		{
			name:    "invalid URL - malformed",
			baseURL: "ht!tp://api.example.com",
			wantErr: "invalid base URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBaseURL(tt.baseURL)
			if tt.wantErr == "" {
				assert.NilError(t, err)
			} else {
				assert.Assert(t, err != nil)
				assert.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}
