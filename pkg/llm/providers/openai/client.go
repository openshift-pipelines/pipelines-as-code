// Package openai is the Client implementation for OpenAI LLM integration.
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/llm/ltypes"
)

const (
	defaultBaseURL = "https://api.openai.com/v1"
	defaultModel   = "gpt-4"

	// Default configuration values.
	defaultTimeoutSeconds = 30
	defaultMaxTokens      = 1000
)

// Config holds the configuration for OpenAI client.
type Config struct {
	APIKey         string
	BaseURL        string
	Model          string
	TimeoutSeconds int
	MaxTokens      int
	HTTPClient     *http.Client
}

// Client implements the LLM interface for OpenAI.
type Client struct {
	config     *Config
	httpClient *http.Client
}

// NewClient creates a new OpenAI client.
func NewClient(config *Config) (*Client, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}

	if config.APIKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	// Set defaults
	if config.BaseURL == "" {
		config.BaseURL = defaultBaseURL
	}
	if config.Model == "" {
		config.Model = defaultModel
	}
	if config.TimeoutSeconds == 0 {
		config.TimeoutSeconds = defaultTimeoutSeconds
	}
	if config.MaxTokens == 0 {
		config.MaxTokens = defaultMaxTokens
	}

	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{}
	}

	if config.TimeoutSeconds > 0 && httpClient.Timeout == 0 {
		httpClient.Timeout = time.Duration(config.TimeoutSeconds) * time.Second
	}

	client := &Client{
		config:     config,
		httpClient: httpClient,
	}

	return client, nil
}

// Analyze sends an analysis request to OpenAI and returns the response.
func (c *Client) Analyze(ctx context.Context, request *ltypes.AnalysisRequest) (*ltypes.AnalysisResponse, error) {
	startTime := time.Now()

	// Build the prompt with context
	fullPrompt, err := c.buildPrompt(request)
	if err != nil {
		return nil, &ltypes.AnalysisError{
			Provider:  c.GetProviderName(),
			Type:      "prompt_build_error",
			Message:   fmt.Sprintf("failed to build prompt: %v", err),
			Retryable: false,
		}
	}

	// Create OpenAI API request
	apiRequest := &openaiRequest{
		Model:     c.config.Model,
		MaxTokens: request.MaxTokens,
		Messages: []openaiMessage{
			{
				Role:    "user",
				Content: fullPrompt,
			},
		},
	}

	// Marshal request
	requestBody, err := json.Marshal(apiRequest)
	if err != nil {
		return nil, &ltypes.AnalysisError{
			Provider:  c.GetProviderName(),
			Type:      "request_marshal_error",
			Message:   fmt.Sprintf("failed to marshal request: %v", err),
			Retryable: false,
		}
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.BaseURL+"/chat/completions", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, &ltypes.AnalysisError{
			Provider:  c.GetProviderName(),
			Type:      "http_request_error",
			Message:   fmt.Sprintf("failed to create HTTP request: %v", err),
			Retryable: false,
		}
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	// Send request
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

	// Parse response
	var apiResponse openaiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return nil, &ltypes.AnalysisError{
			Provider:  c.GetProviderName(),
			Type:      "response_parse_error",
			Message:   fmt.Sprintf("failed to parse response: %v", err),
			Retryable: false,
		}
	}

	// Handle API errors
	if resp.StatusCode != http.StatusOK {
		errorType := "api_error"
		retryable := false

		switch {
		case resp.StatusCode == http.StatusTooManyRequests:
			errorType = "rate_limit_exceeded"
			retryable = true
		case resp.StatusCode == http.StatusUnauthorized:
			errorType = "invalid_api_key"
			retryable = false
		case resp.StatusCode >= 500:
			errorType = "server_error"
			retryable = true
		}

		errorMsg := fmt.Sprintf("OpenAI API error (status %d)", resp.StatusCode)
		if apiResponse.Error != nil {
			errorMsg = fmt.Sprintf("OpenAI API error: %s", apiResponse.Error.Message)
		}

		return nil, &ltypes.AnalysisError{
			Provider:  c.GetProviderName(),
			Type:      errorType,
			Message:   errorMsg,
			Retryable: retryable,
		}
	}

	// Extract content
	if len(apiResponse.Choices) == 0 {
		return nil, &ltypes.AnalysisError{
			Provider:  c.GetProviderName(),
			Type:      "empty_response",
			Message:   "no choices in API response",
			Retryable: false,
		}
	}

	content := apiResponse.Choices[0].Message.Content
	tokensUsed := apiResponse.Usage.TotalTokens

	// Build response
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
	return string(ltypes.LLMProviderOpenAI)
}

// ValidateConfig validates the client configuration.
func (c *Client) ValidateConfig() error {
	if c.config.APIKey == "" {
		return fmt.Errorf("API key is required")
	}
	if c.config.TimeoutSeconds < 0 {
		return fmt.Errorf("timeout seconds must be non-negative")
	}
	if c.config.MaxTokens < 0 {
		return fmt.Errorf("max tokens must be non-negative")
	}
	return nil
}

// buildPrompt combines the base prompt with context data.
func (c *Client) buildPrompt(request *ltypes.AnalysisRequest) (string, error) {
	var promptBuilder strings.Builder

	// Start with the base prompt
	promptBuilder.WriteString(request.Prompt)
	promptBuilder.WriteString("\n\n")

	// Add context sections
	if len(request.Context) > 0 {
		promptBuilder.WriteString("Context Information:\n")

		for key, value := range request.Context {
			promptBuilder.WriteString(fmt.Sprintf("=== %s ===\n", strings.ToUpper(key)))

			switch v := value.(type) {
			case string:
				promptBuilder.WriteString(v)
			case map[string]any, []any:
				jsonData, err := json.MarshalIndent(v, "", "  ")
				if err != nil {
					return "", fmt.Errorf("failed to marshal context %s: %w", key, err)
				}
				promptBuilder.Write(jsonData)
			default:
				promptBuilder.WriteString(fmt.Sprintf("%v", v))
			}

			promptBuilder.WriteString("\n\n")
		}
	}

	return promptBuilder.String(), nil
}

// OpenAI API request/response structures

type openaiRequest struct {
	Model     string          `json:"model"`
	Messages  []openaiMessage `json:"messages"`
	MaxTokens int             `json:"max_tokens,omitempty"`
}

type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []openaiChoice `json:"choices"`
	Usage   openaiUsage    `json:"usage"`
	Error   *openaiError   `json:"error,omitempty"`
}

type openaiChoice struct {
	Index        int           `json:"index"`
	Message      openaiMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

type openaiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type openaiError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}
