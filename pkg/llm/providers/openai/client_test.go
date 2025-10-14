package openai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/llm/ltypes"
	"gotest.tools/v3/assert"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name      string
		config    *Config
		wantError bool
	}{
		{
			name: "valid config",
			config: &Config{
				APIKey:         "test-key",
				BaseURL:        "https://api.openai.com/v1",
				Model:          "gpt-4",
				TimeoutSeconds: 30,
				MaxTokens:      1000,
			},
			wantError: false,
		},
		{
			name: "config with defaults",
			config: &Config{
				APIKey: "test-key",
			},
			wantError: false,
		},
		{
			name:      "nil config",
			config:    nil,
			wantError: true,
		},
		{
			name: "missing api key",
			config: &Config{
				BaseURL: "https://api.openai.com/v1",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.config)

			if tt.wantError {
				assert.Assert(t, err != nil, "expected error but got none")
				assert.Assert(t, client == nil, "expected nil client on error")
			} else {
				assert.NilError(t, err)
				assert.Assert(t, client != nil, "expected non-nil client")
				assert.Equal(t, client.GetProviderName(), "openai")
			}
		})
	}
}

func TestClient_ValidateConfig(t *testing.T) {
	tests := []struct {
		name      string
		config    *Config
		wantError bool
	}{
		{
			name: "valid config",
			config: &Config{
				APIKey:         "test-key",
				TimeoutSeconds: 30,
				MaxTokens:      1000,
			},
			wantError: false,
		},
		{
			name: "missing api key",
			config: &Config{
				TimeoutSeconds: 30,
				MaxTokens:      1000,
			},
			wantError: true,
		},
		{
			name: "negative timeout",
			config: &Config{
				APIKey:         "test-key",
				TimeoutSeconds: -1,
				MaxTokens:      1000,
			},
			wantError: true,
		},
		{
			name: "negative max tokens",
			config: &Config{
				APIKey:         "test-key",
				TimeoutSeconds: 30,
				MaxTokens:      -1,
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, _ := NewClient(&Config{APIKey: "test"}) // Create a client to test validation
			client.config = tt.config                       // Override config for testing

			err := client.ValidateConfig()

			if tt.wantError {
				assert.Assert(t, err != nil, "expected error but got none")
			} else {
				assert.NilError(t, err)
			}
		})
	}
}

func TestClient_Analyze(t *testing.T) {
	tests := []struct {
		name           string
		request        *ltypes.AnalysisRequest
		mockResponse   string
		mockStatusCode int
		wantError      bool
		checkResponse  func(t *testing.T, resp *ltypes.AnalysisResponse)
	}{
		{
			name: "successful analysis",
			request: &ltypes.AnalysisRequest{
				Prompt:         "Analyze this error",
				Context:        map[string]any{"error": "test error"},
				MaxTokens:      100,
				TimeoutSeconds: 30,
			},
			mockResponse: `{
                "id": "test-id",
                "object": "chat.completion",
                "choices": [
                    {
                        "index": 0,
						"message": {
							"role": "assistant",
							"content": "This is a test analysis response"
						},
						"finish_reason": "stop"
					}
				],
				"usage": {
					"prompt_tokens": 10,
					"completion_tokens": 20,
					"total_tokens": 30
				}
			}`,
			mockStatusCode: http.StatusOK,
			wantError:      false,
			checkResponse: func(t *testing.T, resp *ltypes.AnalysisResponse) {
				assert.Equal(t, resp.Content, "This is a test analysis response")
				assert.Equal(t, resp.TokensUsed, 30)
				assert.Equal(t, resp.Provider, "openai")
				assert.Assert(t, !resp.Timestamp.IsZero())
				assert.Assert(t, resp.Duration > 0)
			},
		},
		{
			name: "json output analysis",
			request: &ltypes.AnalysisRequest{
				Prompt:         "Analyze this error",
				Context:        map[string]any{"error": "test error"},
				MaxTokens:      100,
				TimeoutSeconds: 30,
			},
			mockResponse: `{
				"id": "test-id",
				"object": "chat.completion",
				"choices": [
					{
						"index": 0,
						"message": {
							"role": "assistant",
							"content": "{\"analysis\": \"test\", \"score\": 5}"
						},
						"finish_reason": "stop"
					}
				],
				"usage": {
					"prompt_tokens": 10,
					"completion_tokens": 20,
					"total_tokens": 30
				}
			}`,
			mockStatusCode: http.StatusOK,
			wantError:      false,
			checkResponse:  func(_ *testing.T, _ *ltypes.AnalysisResponse) {},
		},
		{
			name: "api error response",
			request: &ltypes.AnalysisRequest{
				Prompt:    "Analyze this error",
				MaxTokens: 100,
			},
			mockResponse: `{
				"error": {
					"message": "Invalid API key",
					"type": "invalid_request_error",
					"code": "invalid_api_key"
				}
			}`,
			mockStatusCode: http.StatusUnauthorized,
			wantError:      true,
		},
		{
			name: "rate limit error",
			request: &ltypes.AnalysisRequest{
				Prompt:    "Analyze this error",
				MaxTokens: 100,
			},
			mockResponse: `{
				"error": {
					"message": "Rate limit exceeded",
					"type": "rate_limit_error"
				}
			}`,
			mockStatusCode: http.StatusTooManyRequests,
			wantError:      true,
		},
		{
			name: "empty choices response",
			request: &ltypes.AnalysisRequest{
				Prompt:    "Analyze this error",
				MaxTokens: 100,
			},
			mockResponse: `{
				"id": "test-id",
				"object": "chat.completion",
				"choices": [],
				"usage": {
					"total_tokens": 0
				}
			}`,
			mockStatusCode: http.StatusOK,
			wantError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := makeMockHTTPClient(tt.mockStatusCode, tt.mockResponse, func(r *http.Request) {
				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, r.Header.Get("Content-Type"), "application/json")
				assert.Assert(t, r.Header.Get("Authorization") != "", "expected authorization header")

				var reqBody openaiRequest
				err := json.NewDecoder(r.Body).Decode(&reqBody)
				assert.NilError(t, err)
				assert.Equal(t, reqBody.Model, defaultModel)
				assert.Assert(t, len(reqBody.Messages) > 0, "expected messages in request")
			})

			client, err := NewClient(&Config{
				APIKey:     "test-key",
				HTTPClient: mockClient,
			})
			assert.NilError(t, err)

			ctx := context.Background()
			resp, err := client.Analyze(ctx, tt.request)

			if tt.wantError {
				assert.Assert(t, err != nil, "expected error but got none")
				assert.Assert(t, resp == nil, "expected nil response on error")
			} else {
				assert.NilError(t, err)
				assert.Assert(t, resp != nil, "expected non-nil response")
				if tt.checkResponse != nil {
					tt.checkResponse(t, resp)
				}
			}
		})
	}
}

func TestClient_BuildPrompt(t *testing.T) {
	client, err := NewClient(&Config{APIKey: "test-key"})
	assert.NilError(t, err)

	tests := []struct {
		name            string
		request         *ltypes.AnalysisRequest
		expectedContent []string // Strings that should be present in the prompt
	}{
		{
			name: "simple prompt",
			request: &ltypes.AnalysisRequest{
				Prompt:  "Analyze this",
				Context: map[string]any{},
			},
			expectedContent: []string{"Analyze this"},
		},
		{
			name: "prompt with context",
			request: &ltypes.AnalysisRequest{
				Prompt: "Analyze this error",
				Context: map[string]any{
					"error":  "test error message",
					"status": "failed",
				},
			},
			expectedContent: []string{"Analyze this error", "ERROR", "test error message", "STATUS", "failed"},
		},
		{
			name: "prompt with complex context",
			request: &ltypes.AnalysisRequest{
				Prompt: "Review this pipeline",
				Context: map[string]any{
					"pipeline": map[string]any{
						"name":   "test-pipeline",
						"status": "failed",
					},
					"logs": []string{"line 1", "line 2"},
				},
			},
			expectedContent: []string{"Review this pipeline", "PIPELINE", "test-pipeline", "LOGS"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt, err := client.buildPrompt(tt.request)
			assert.NilError(t, err)

			for _, expected := range tt.expectedContent {
				assert.Assert(t, strings.Contains(prompt, expected),
					"expected prompt to contain '%s', got: %s", expected, prompt)
			}
		})
	}
}

func TestClient_Timeout(t *testing.T) {
	slowTransport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		select {
		case <-time.After(100 * time.Millisecond):
			body := `{"choices": [{"message": {"content": "response"}}], "usage": {"total_tokens": 10}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
	})

	customClient := &http.Client{
		Timeout:   50 * time.Millisecond,
		Transport: slowTransport,
	}

	client, err := NewClient(&Config{
		APIKey:     "test-key",
		HTTPClient: customClient,
	})
	assert.NilError(t, err)

	ctx := context.Background()
	request := &ltypes.AnalysisRequest{
		Prompt:    "test",
		MaxTokens: 100,
	}

	_, err = client.Analyze(ctx, request)
	assert.Assert(t, err != nil, "expected timeout error")

	var analysisErr *ltypes.AnalysisError
	assert.Assert(t, errors.As(err, &analysisErr), "expected AnalysisError")
	assert.Equal(t, analysisErr.Type, "http_error")
	assert.Assert(t, analysisErr.Retryable, "timeout errors should be retryable")
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func makeMockHTTPClient(statusCode int, body string, onRequest func(*http.Request)) *http.Client {
	return &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if onRequest != nil {
				onRequest(req)
			}

			resp := &http.Response{
				StatusCode: statusCode,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}
			resp.Header.Set("Content-Type", "application/json")
			return resp, nil
		}),
	}
}
