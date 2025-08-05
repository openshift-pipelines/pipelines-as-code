package github

import (
	"testing"
)

func TestParseAPICallLog(t *testing.T) {
	// Test with the real log format from the user's example
	testCases := []struct {
		name     string
		logLine  string
		expected *InstrumentationAPICall
	}{
		{
			name:    "parse valid JSON log line",
			logLine: `API Call 1: {"level":"debug","ts":"2025-08-05T16:12:17.508Z","logger":"pipelinesascode","caller":"github/profiler.go:131","msg":"GitHub API call completed","commit":"bacf698","provider":"github","event-id":"f4698b50-7216-11f0-9c6b-443ea2de733f","event-sha":"62a0b25ea7bdc3ef0e8789abff8cd797ab6cac25","event-type":"no-ops-comment","source-repo-url":"https://ghe.pipelinesascode.com/chmouel/e2e-gapps","target-branch":"main","source-branch":"pac-e2e-test-mf7r6","namespace":"pac-e2e-ns-jhh9f","operation":"get_commit","duration_ms":156,"provider":"github","repo":"pac-e2e-ns-jhh9f/pac-e2e-ns-jhh9f","url_path":"/api/v3/repos/chmouel/e2e-gapps/git/commits/62a0b25ea7bdc3ef0e8789abff8cd797ab6cac25","rate_limit_remaining":"","status_code":200}`,
			expected: &InstrumentationAPICall{
				Operation:          "get_commit",
				DurationMs:         156,
				URLPath:            "/api/v3/repos/chmouel/e2e-gapps/git/commits/62a0b25ea7bdc3ef0e8789abff8cd797ab6cac25",
				RateLimitRemaining: "",
				StatusCode:         200,
				Provider:           "github",
				Repo:               "pac-e2e-ns-jhh9f/pac-e2e-ns-jhh9f",
			},
		},
		{
			name:    "parse log line with rate limit",
			logLine: `API Call 2: {"level":"debug","ts":"2025-08-05T16:12:17.665Z","logger":"pipelinesascode","caller":"github/profiler.go:131","msg":"GitHub API call completed","operation":"get_root_tree","duration_ms":157,"provider":"github","repo":"pac-e2e-ns-jhh9f/pac-e2e-ns-jhh9f","url_path":"/api/v3/repos/chmouel/e2e-gapps/git/trees/62a0b25ea7bdc3ef0e8789abff8cd797ab6cac25","rate_limit_remaining":"4999","status_code":200}`,
			expected: &InstrumentationAPICall{
				Operation:          "get_root_tree",
				DurationMs:         157,
				URLPath:            "/api/v3/repos/chmouel/e2e-gapps/git/trees/62a0b25ea7bdc3ef0e8789abff8cd797ab6cac25",
				RateLimitRemaining: "4999",
				StatusCode:         200,
				Provider:           "github",
				Repo:               "pac-e2e-ns-jhh9f/pac-e2e-ns-jhh9f",
			},
		},
		{
			name:     "parse invalid log line",
			logLine:  "This is not a valid log line",
			expected: nil,
		},
		{
			name:     "parse log line without JSON",
			logLine:  "API Call 1: This is not JSON",
			expected: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := parseAPICallLog(tc.logLine)

			if tc.expected == nil {
				if result != nil {
					t.Errorf("Expected nil result, got %+v", result)
				}
				return
			}

			if result == nil {
				t.Errorf("Expected result, got nil")
				return
			}

			if result.Operation != tc.expected.Operation {
				t.Errorf("Expected operation %s, got %s", tc.expected.Operation, result.Operation)
			}
			if result.DurationMs != tc.expected.DurationMs {
				t.Errorf("Expected duration_ms %d, got %d", tc.expected.DurationMs, result.DurationMs)
			}
			if result.URLPath != tc.expected.URLPath {
				t.Errorf("Expected url_path %s, got %s", tc.expected.URLPath, result.URLPath)
			}
			if result.RateLimitRemaining != tc.expected.RateLimitRemaining {
				t.Errorf("Expected rate_limit_remaining %s, got %s", tc.expected.RateLimitRemaining, result.RateLimitRemaining)
			}
			if result.StatusCode != tc.expected.StatusCode {
				t.Errorf("Expected status_code %d, got %d", tc.expected.StatusCode, result.StatusCode)
			}
			if result.Provider != tc.expected.Provider {
				t.Errorf("Expected provider %s, got %s", tc.expected.Provider, result.Provider)
			}
			if result.Repo != tc.expected.Repo {
				t.Errorf("Expected repo %s, got %s", tc.expected.Repo, result.Repo)
			}
		})
	}
}
