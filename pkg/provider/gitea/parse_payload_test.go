package gitea

import (
	"context"
	"net/http"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
	"gotest.tools/v3/assert"
)

func TestParsePayloadIssueCommentPullRequestData(t *testing.T) {
	const payloadWithPR = `{
		"action": "created",
		"issue": {
			"url": "https://gitea.example/api/v1/repos/test-org/test-repo/issues/42",
			"number": 42,
			"pull_request": {"html_url": "https://gitea.example/test-org/test-repo/pulls/42"}
		},
		"pull_request": {
			"number": 42,
			"title": "Test PR",
			"head": {
				"ref": "feature",
				"sha": "abc123",
				"repo": {"html_url": "https://gitea.example/testuser/test-repo-fork"}
			},
			"base": {
				"ref": "main",
				"repo": {"html_url": "https://gitea.example/test-org/test-repo"}
			},
			"html_url": "https://gitea.example/test-org/test-repo/pulls/42"
		},
		"comment": {"body": "/retest"},
		"repository": {
			"name": "test-repo",
			"owner": {"login": "test-org"},
			"html_url": "https://gitea.example/test-org/test-repo",
			"default_branch": "main"
		},
		"sender": {"login": "testuser"}
	}`

	tests := []struct {
		name         string
		eventType    string
		payload      string
		wantErr      string
		wantHeadURL  string
		wantBaseURL  string
		wantPRNumber int
		wantSHA      string
	}{
		{
			name:         "issue_comment populates source and target urls",
			eventType:    "issue_comment",
			payload:      payloadWithPR,
			wantHeadURL:  "https://gitea.example/testuser/test-repo-fork",
			wantBaseURL:  "https://gitea.example/test-org/test-repo",
			wantPRNumber: 42,
			wantSHA:      "abc123",
		},
		{
			name:         "pull_request_comment populates source and target urls",
			eventType:    "pull_request_comment",
			payload:      payloadWithPR,
			wantHeadURL:  "https://gitea.example/testuser/test-repo-fork",
			wantBaseURL:  "https://gitea.example/test-org/test-repo",
			wantPRNumber: 42,
			wantSHA:      "abc123",
		},
		{
			name:      "non pull request issue comment still fails",
			eventType: "issue_comment",
			payload: `{
				"action": "created",
				"issue": {"url": "https://gitea.example/api/v1/repos/test-org/test-repo/issues/7"},
				"comment": {"body": "/retest"},
				"repository": {"name": "test-repo", "owner": {"login": "test-org"}},
				"sender": {"login": "testuser"}
			}`,
			wantErr: "issue comment is not coming from a pull_request",
		},
		{
			name:      "missing head repo does not panic and leaves head url empty",
			eventType: "issue_comment",
			payload: `{
				"action": "created",
				"issue": {
					"url": "https://gitea.example/api/v1/repos/test-org/test-repo/issues/44",
					"pull_request": {"html_url": "https://gitea.example/test-org/test-repo/pulls/44"}
				},
				"pull_request": {
					"number": 44,
					"head": {"ref": "feature", "sha": "abc999"},
					"base": {"ref": "main", "repo": {"html_url": "https://gitea.example/test-org/test-repo"}},
					"html_url": "https://gitea.example/test-org/test-repo/pulls/44"
				},
				"comment": {"body": "/retest"},
				"repository": {"name": "test-repo", "owner": {"login": "test-org"}}
			}`,
			wantHeadURL:  "",
			wantBaseURL:  "https://gitea.example/test-org/test-repo",
			wantPRNumber: 44,
			wantSHA:      "abc999",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{Header: http.Header{"X-Gitea-Event-Type": []string{tt.eventType}}}
			got, err := (&Provider{}).ParsePayload(context.Background(), nil, req, tt.payload)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			assert.NilError(t, err)
			assert.Assert(t, got != nil)
			assert.Equal(t, got.TriggerTarget, triggertype.PullRequest)
			assert.Equal(t, got.PullRequestNumber, tt.wantPRNumber)
			assert.Equal(t, got.HeadURL, tt.wantHeadURL)
			assert.Equal(t, got.BaseURL, tt.wantBaseURL)
			assert.Equal(t, got.SHA, tt.wantSHA)
		})
	}
}
