// Package gemini is the Client implementation for Google Gemini LLM integration.
package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/llm/ltypes"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/llm/providers"
)

const (
	defaultBaseURL = "https://generativelanguage.googleapis.com/v1beta"
	defaultModel   = "gemini-2.5-flash-lite"

	// Default configuration values.
	defaultTimeoutSeconds = 30
	defaultMaxTokens      = 1000
)

// Config holds the configuration for Gemini client.
type Config struct {
	APIKey         string
	BaseURL        string
	Model          string
	TimeoutSeconds int
	MaxTokens      int
}

// Client implements the LLM interface for Google Gemini.
type Client struct {
	config     *Config
	httpClient *http.Client
}

// NewClient creates a new Gemini client.
func NewClient(config *Config) (*Client, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}

	if config.APIKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	commonCfg := &providers.CommonConfig{
		APIKey:         config.APIKey,
		TimeoutSeconds: config.TimeoutSeconds,
		MaxTokens:      config.MaxTokens,
	}
	if err := providers.ApplyDefaults(commonCfg); err != nil {
		return nil, err
	}

	config.TimeoutSeconds = commonCfg.TimeoutSeconds
	config.MaxTokens = commonCfg.MaxTokens

	if config.BaseURL == "" {
		config.BaseURL = defaultBaseURL
	}
	if config.Model == "" {
		config.Model = defaultModel
	}

	client := &Client{
		config: config,
		httpClient: &http.Client{
			Timeout: time.Duration(config.TimeoutSeconds) * time.Second,
		},
	}

	return client, nil
}

// Analyze sends an analysis request to Gemini and returns the response.
func (c *Client) Analyze(ctx context.Context, request *ltypes.AnalysisRequest) (*ltypes.AnalysisResponse, error) {
	startTime := time.Now()

	fullPrompt, err := providers.BuildPrompt(request)
	if err != nil {
		return nil, &ltypes.AnalysisError{
			Provider:  c.GetProviderName(),
			Type:      "prompt_build_error",
			Message:   fmt.Sprintf("failed to build prompt: %v", err),
			Retryable: false,
		}
	}

	apiRequest := &geminiRequest{
		Contents: []geminiContent{
			{
				Parts: []geminiPart{
					{
						Text: fullPrompt,
					},
				},
			},
		},
		GenerationConfig: &geminiGenerationConfig{
			MaxOutputTokens: request.MaxTokens,
		},
	}

	requestBody, err := json.Marshal(apiRequest)
	if err != nil {
		return nil, &ltypes.AnalysisError{
			Provider:  c.GetProviderName(),
			Type:      "request_marshal_error",
			Message:   fmt.Sprintf("failed to marshal request: %v", err),
			Retryable: false,
		}
	}

	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", c.config.BaseURL, c.config.Model, c.config.APIKey)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, &ltypes.AnalysisError{
			Provider:  c.GetProviderName(),
			Type:      "http_request_error",
			Message:   fmt.Sprintf("failed to create HTTP request: %v", err),
			Retryable: false,
		}
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, &ltypes.AnalysisError{
			Provider:  c.GetProviderName(),
			Type:      "http_error",
			Message:   fmt.Sprintf("HTTP request failed: %v", err),
			Retryable: true,
		}
	}
	defer resp.Body.Close()

	var apiResponse geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return nil, &ltypes.AnalysisError{
			Provider:  c.GetProviderName(),
			Type:      "response_parse_error",
			Message:   fmt.Sprintf("failed to parse response: %v", err),
			Retryable: false,
		}
	}

	if resp.StatusCode != http.StatusOK {
		errorType := "api_error"
		retryable := false

		switch {
		case resp.StatusCode == http.StatusTooManyRequests:
			errorType = "rate_limit_exceeded"
			retryable = true
		case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
			errorType = "invalid_api_key"
			retryable = false
		case resp.StatusCode >= 500:
			errorType = "server_error"
			retryable = true
		}

		errorMsg := fmt.Sprintf("Gemini API error (status %d)", resp.StatusCode)
		if apiResponse.Error != nil {
			errorMsg = fmt.Sprintf("Gemini API error: %s", apiResponse.Error.Message)
		}

		return nil, &ltypes.AnalysisError{
			Provider:  c.GetProviderName(),
			Type:      errorType,
			Message:   errorMsg,
			Retryable: retryable,
		}
	}

	if len(apiResponse.Candidates) == 0 {
		return nil, &ltypes.AnalysisError{
			Provider:  c.GetProviderName(),
			Type:      "empty_response",
			Message:   "no candidates in API response",
			Retryable: false,
		}
	}

	candidate := apiResponse.Candidates[0]
	if len(candidate.Content.Parts) == 0 {
		return nil, &ltypes.AnalysisError{
			Provider:  c.GetProviderName(),
			Type:      "empty_response",
			Message:   "no content parts in API response",
			Retryable: false,
		}
	}

	content := candidate.Content.Parts[0].Text

	tokensUsed := len(strings.Fields(content + fullPrompt))

	response := &ltypes.AnalysisResponse{
		Content:    content,
		TokensUsed: tokensUsed,
		Provider:   c.GetProviderName(),
		Timestamp:  time.Now(),
		Duration:   time.Since(startTime),
	}

	return response, nil
}

// GetProviderName returns the provider name.
func (c *Client) GetProviderName() string {
	return string(ltypes.LLMProviderGemini)
}

// ValidateConfig validates the client configuration.
func (c *Client) ValidateConfig() error {
	commonCfg := &providers.CommonConfig{
		APIKey:         c.config.APIKey,
		TimeoutSeconds: c.config.TimeoutSeconds,
		MaxTokens:      c.config.MaxTokens,
	}
	if err := providers.ValidateCommonConfig(commonCfg); err != nil {
		return err
	}

	if err := providers.ValidateBaseURL(c.config.BaseURL); err != nil {
		return err
	}

	return nil
}

// Gemini API request/response structures

type geminiRequest struct {
	Contents         []geminiContent         `json:"contents"`
	GenerationConfig *geminiGenerationConfig `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenerationConfig struct {
	MaxOutputTokens int `json:"maxOutputTokens,omitempty"`
}

type geminiResponse struct {
	Candidates []geminiCandidate `json:"candidates"`
	Error      *geminiError      `json:"error,omitempty"`
}

type geminiCandidate struct {
	Content       geminiContent `json:"content"`
	FinishReason  string        `json:"finishReason"`
	SafetyRatings []any         `json:"safetyRatings"`
}

type geminiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}
