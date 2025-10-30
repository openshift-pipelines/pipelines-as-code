package providers

import (
	"strings"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/llm/ltypes"
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

func TestBuildPrompt(t *testing.T) {
	tests := []struct {
		name        string
		request     *ltypes.AnalysisRequest
		wantContain []string
		wantErr     bool
	}{
		{
			name: "simple prompt without context",
			request: &ltypes.AnalysisRequest{
				Prompt: "Analyze this error",
			},
			wantContain: []string{"Analyze this error"},
			wantErr:     false,
		},
		{
			name: "prompt with string context",
			request: &ltypes.AnalysisRequest{
				Prompt: "Analyze",
				Context: map[string]any{
					"logs": "error log content",
				},
			},
			wantContain: []string{"Analyze", "Context Information:", "=== LOGS ===", "error log content"},
			wantErr:     false,
		},
		{
			name: "prompt with map context",
			request: &ltypes.AnalysisRequest{
				Prompt: "Analyze",
				Context: map[string]any{
					"metadata": map[string]any{
						"name": "test",
						"id":   123,
					},
				},
			},
			wantContain: []string{"Analyze", "Context Information:", "=== METADATA ===", "\"name\"", "\"test\""},
			wantErr:     false,
		},
		{
			name: "prompt with array context",
			request: &ltypes.AnalysisRequest{
				Prompt: "Analyze",
				Context: map[string]any{
					"items": []any{"item1", "item2", "item3"},
				},
			},
			wantContain: []string{"Analyze", "Context Information:", "=== ITEMS ===", "item1", "item2"},
			wantErr:     false,
		},
		{
			name: "prompt with multiple context keys",
			request: &ltypes.AnalysisRequest{
				Prompt: "Analyze",
				Context: map[string]any{
					"logs":     "error logs here",
					"metadata": map[string]any{"version": "1.0"},
				},
			},
			wantContain: []string{"Analyze", "Context Information:", "=== LOGS ===", "=== METADATA ==="},
			wantErr:     false,
		},
		{
			name: "prompt with number context",
			request: &ltypes.AnalysisRequest{
				Prompt: "Analyze",
				Context: map[string]any{
					"count": 42,
				},
			},
			wantContain: []string{"Analyze", "=== COUNT ===", "42"},
			wantErr:     false,
		},
		{
			name: "prompt with boolean context",
			request: &ltypes.AnalysisRequest{
				Prompt: "Analyze",
				Context: map[string]any{
					"success": false,
				},
			},
			wantContain: []string{"Analyze", "=== SUCCESS ===", "false"},
			wantErr:     false,
		},
		{
			name: "empty prompt with context",
			request: &ltypes.AnalysisRequest{
				Prompt: "",
				Context: map[string]any{
					"logs": "content",
				},
			},
			wantContain: []string{"Context Information:", "=== LOGS ==="},
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt, err := BuildPrompt(tt.request)

			if tt.wantErr {
				assert.Assert(t, err != nil)
			} else {
				assert.NilError(t, err)
				for _, want := range tt.wantContain {
					assert.Assert(t, strings.Contains(prompt, want),
						"prompt should contain %q, got: %s", want, prompt)
				}
			}
		})
	}
}

func TestBuildPromptError(t *testing.T) {
	tests := []struct {
		name    string
		request *ltypes.AnalysisRequest
		errMsg  string
	}{
		{
			name: "unmarshalable channel in nested map",
			request: &ltypes.AnalysisRequest{
				Prompt: "Test",
				Context: map[string]any{
					"nested": map[string]any{
						"bad": make(chan int),
					},
				},
			},
			errMsg: "failed to marshal context nested",
		},
		{
			name: "unmarshalable function in nested map",
			request: &ltypes.AnalysisRequest{
				Prompt: "Test",
				Context: map[string]any{
					"data": map[string]any{
						"fn": func() {},
					},
				},
			},
			errMsg: "failed to marshal context data",
		},
		{
			name: "unmarshalable channel in array",
			request: &ltypes.AnalysisRequest{
				Prompt: "Test",
				Context: map[string]any{
					"items": []any{make(chan int)},
				},
			},
			errMsg: "failed to marshal context items",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := BuildPrompt(tt.request)
			assert.Assert(t, err != nil)
			assert.ErrorContains(t, err, tt.errMsg)
		})
	}
}

func TestBuildPromptContextOrdering(t *testing.T) {
	// Test that context is properly formatted with headers
	request := &ltypes.AnalysisRequest{
		Prompt: "Base prompt",
		Context: map[string]any{
			"logs": "log content",
		},
	}

	prompt, err := BuildPrompt(request)
	assert.NilError(t, err)

	// Verify the order: base prompt comes first, then context
	baseIdx := strings.Index(prompt, "Base prompt")
	contextIdx := strings.Index(prompt, "Context Information:")
	logsIdx := strings.Index(prompt, "=== LOGS ===")

	assert.Assert(t, baseIdx >= 0, "should contain base prompt")
	assert.Assert(t, contextIdx >= 0, "should contain context header")
	assert.Assert(t, logsIdx >= 0, "should contain logs header")
	assert.Assert(t, baseIdx < contextIdx, "base prompt should come before context")
	assert.Assert(t, contextIdx < logsIdx, "context header should come before logs")
}
