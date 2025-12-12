package cel

import (
	"strings"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"gotest.tools/v3/assert"
)

func TestGitHubParserWithMissingFields(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		wantErr bool
		checks  func(t *testing.T, event *info.Event)
	}{
		{
			name: "missing repository field",
			payload: `{
				"action": "opened",
				"pull_request": {
					"number": 1,
					"title": "Test PR",
					"head": {"ref": "feature", "sha": "abc123"},
					"base": {"ref": "main"}
				},
				"sender": {"login": "testuser"}
			}`,
			wantErr: false,
			checks: func(t *testing.T, event *info.Event) {
				assert.Equal(t, event.Organization, "")
				assert.Equal(t, event.Repository, "")
				assert.Equal(t, event.Sender, "testuser")
				assert.Equal(t, event.PullRequestNumber, 1)
			},
		},
		{
			name: "missing pull request field",
			payload: `{
				"action": "opened",
				"repository": {
					"name": "test-repo",
					"owner": {"login": "test-org"},
					"html_url": "https://github.com/test-org/test-repo"
				},
				"sender": {"login": "testuser"}
			}`,
			wantErr: false,
			checks: func(t *testing.T, event *info.Event) {
				assert.Equal(t, event.Organization, "test-org")
				assert.Equal(t, event.Repository, "test-repo")
				assert.Equal(t, event.Sender, "testuser")
				assert.Equal(t, event.SHA, "")
				assert.Equal(t, event.PullRequestNumber, 0)
			},
		},
		{
			name: "missing sender field",
			payload: `{
				"action": "opened",
				"pull_request": {
					"number": 1,
					"title": "Test PR",
					"head": {"ref": "feature", "sha": "abc123"},
					"base": {"ref": "main"}
				},
				"repository": {
					"name": "test-repo",
					"owner": {"login": "test-org"}
				}
			}`,
			wantErr: false,
			checks: func(t *testing.T, event *info.Event) {
				assert.Equal(t, event.Sender, "")
				assert.Equal(t, event.Organization, "test-org")
			},
		},
		{
			name: "missing owner in repository",
			payload: `{
				"action": "opened",
				"pull_request": {
					"number": 1,
					"title": "Test PR",
					"head": {"ref": "feature", "sha": "abc123"},
					"base": {"ref": "main"}
				},
				"repository": {"name": "test-repo"},
				"sender": {"login": "testuser"}
			}`,
			wantErr: false,
			checks: func(t *testing.T, event *info.Event) {
				assert.Equal(t, event.Organization, "")
				assert.Equal(t, event.Repository, "test-repo")
			},
		},
		{
			name: "missing head in pull request",
			payload: `{
				"action": "opened",
				"pull_request": {
					"number": 1,
					"title": "Test PR",
					"base": {"ref": "main"}
				},
				"repository": {
					"name": "test-repo",
					"owner": {"login": "test-org"}
				},
				"sender": {"login": "testuser"}
			}`,
			wantErr: false,
			checks: func(t *testing.T, event *info.Event) {
				assert.Equal(t, event.SHA, "")
				assert.Equal(t, event.HeadBranch, "")
				assert.Equal(t, event.BaseBranch, "main")
			},
		},
		{
			name: "missing base in pull request",
			payload: `{
				"action": "opened",
				"pull_request": {
					"number": 1,
					"title": "Test PR",
					"head": {"ref": "feature", "sha": "abc123"}
				},
				"repository": {
					"name": "test-repo",
					"owner": {"login": "test-org"}
				},
				"sender": {"login": "testuser"}
			}`,
			wantErr: false,
			checks: func(t *testing.T, event *info.Event) {
				assert.Equal(t, event.HeadBranch, "feature")
				assert.Equal(t, event.BaseBranch, "")
			},
		},
		{
			name: "missing head repo in pull request",
			payload: `{
				"action": "opened",
				"pull_request": {
					"number": 1,
					"title": "Test PR",
					"head": {"ref": "feature", "sha": "abc123"},
					"base": {"ref": "main"}
				},
				"repository": {
					"name": "test-repo",
					"owner": {"login": "test-org"}
				},
				"sender": {"login": "testuser"}
			}`,
			wantErr: false,
			checks: func(t *testing.T, event *info.Event) {
				assert.Equal(t, event.HeadURL, "")
			},
		},
		{
			name: "push event missing head commit",
			payload: `{
				"ref": "refs/heads/main",
				"repository": {
					"name": "test-repo",
					"owner": {"login": "test-org"},
					"html_url": "https://github.com/test-org/test-repo"
				},
				"sender": {"login": "testuser"}
			}`,
			wantErr: false,
			checks: func(t *testing.T, event *info.Event) {
				assert.Equal(t, event.SHA, "")
				assert.Equal(t, event.SHAURL, "")
				assert.Equal(t, event.Organization, "test-org")
				assert.Equal(t, event.HeadBranch, "refs/heads/main")
			},
		},
		{
			name: "issue comment with issue but no pull request links",
			payload: `{
				"action": "created",
				"issue": {
					"number": 1
				},
				"comment": {"body": "/retest"},
				"repository": {
					"name": "test-repo",
					"owner": {"login": "test-org"}
				},
				"sender": {"login": "testuser"}
			}`,
			wantErr: true,
		},
		{
			name: "commit comment event missing comment",
			payload: `{
				"action": "created",
				"repository": {
					"name": "test-repo",
					"owner": {"login": "test-org"}
				},
				"sender": {"login": "testuser"}
			}`,
			wantErr: false,
			checks: func(t *testing.T, event *info.Event) {
				assert.Equal(t, event.SHA, "")
				assert.Equal(t, event.TriggerComment, "")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := map[string]string{
				"X-GitHub-Event": "pull_request",
			}
			// Determine event type from payload based on test name
			switch {
			case strings.Contains(tt.name, "push"):
				headers["X-GitHub-Event"] = "push"
			case strings.Contains(tt.name, "issue comment"):
				headers["X-GitHub-Event"] = "issue_comment"
			case strings.Contains(tt.name, "commit comment"):
				headers["X-GitHub-Event"] = "commit_comment"
			}

			event, err := eventFromGitHub([]byte(tt.payload), headers)
			if tt.wantErr {
				assert.Assert(t, err != nil)
				return
			}
			assert.NilError(t, err)
			assert.Assert(t, event != nil)
			if tt.checks != nil {
				tt.checks(t, event)
			}
		})
	}
}

func TestGiteaParserWithMissingFields(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		wantErr bool
		checks  func(t *testing.T, event *info.Event)
	}{
		{
			name: "pull request with missing repository",
			payload: `{
				"action": "opened",
				"number": 1,
				"pull_request": {
					"title": "Test PR",
					"head": {"ref": "feature", "sha": "abc123"},
					"base": {"ref": "main"}
				},
				"sender": {"login": "testuser"}
			}`,
			wantErr: false,
			checks: func(t *testing.T, event *info.Event) {
				assert.Equal(t, event.Organization, "")
				assert.Equal(t, event.Repository, "")
				assert.Equal(t, event.Sender, "testuser")
			},
		},
		{
			name: "pull request with missing sender",
			payload: `{
				"action": "opened",
				"number": 1,
				"pull_request": {
					"title": "Test PR",
					"head": {"ref": "feature", "sha": "abc123"},
					"base": {"ref": "main"}
				},
				"repository": {
					"name": "test-repo",
					"owner": {"login": "test-org"}
				}
			}`,
			wantErr: false,
			checks: func(t *testing.T, event *info.Event) {
				assert.Equal(t, event.Sender, "")
				assert.Equal(t, event.Repository, "test-repo")
			},
		},
		{
			name: "pull request with missing pull request field",
			payload: `{
				"action": "opened",
				"number": 1,
				"repository": {
					"name": "test-repo",
					"owner": {"login": "test-org"}
				},
				"sender": {"login": "testuser"}
			}`,
			wantErr: false,
			checks: func(t *testing.T, event *info.Event) {
				assert.Equal(t, event.SHA, "")
				assert.Equal(t, event.HeadBranch, "")
			},
		},
		{
			name: "pull request with missing owner",
			payload: `{
				"action": "opened",
				"number": 1,
				"pull_request": {
					"title": "Test PR",
					"head": {"ref": "feature", "sha": "abc123"},
					"base": {"ref": "main"}
				},
				"repository": {"name": "test-repo"},
				"sender": {"login": "testuser"}
			}`,
			wantErr: false,
			checks: func(t *testing.T, event *info.Event) {
				assert.Equal(t, event.Organization, "")
				assert.Equal(t, event.Repository, "test-repo")
			},
		},
		{
			name: "push with missing repo",
			payload: `{
				"ref": "refs/heads/main",
				"head_commit": {
					"id": "abc123",
					"message": "Test commit"
				},
				"sender": {"login": "testuser"}
			}`,
			wantErr: false,
			checks: func(t *testing.T, event *info.Event) {
				assert.Equal(t, event.Organization, "")
				assert.Equal(t, event.Repository, "")
				assert.Equal(t, event.SHA, "abc123")
			},
		},
		{
			name: "push with missing head commit",
			payload: `{
				"ref": "refs/heads/main",
				"before": "def456",
				"repository": {
					"name": "test-repo",
					"owner": {"login": "test-org"}
				},
				"sender": {"login": "testuser"}
			}`,
			wantErr: false,
			checks: func(t *testing.T, event *info.Event) {
				assert.Equal(t, event.SHA, "def456")
				assert.Equal(t, event.SHAURL, "")
			},
		},
		{
			name: "issue comment with issue but no pull request",
			payload: `{
				"action": "created",
				"issue": {"number": 1},
				"comment": {"body": "/retest"},
				"repository": {
					"name": "test-repo",
					"owner": {"login": "test-org"}
				},
				"sender": {"login": "testuser"}
			}`,
			wantErr: true,
		},
		{
			name: "issue comment with missing comment",
			payload: `{
				"action": "created",
				"issue": {
					"number": 1,
					"pull_request": {}
				},
				"repository": {
					"name": "test-repo",
					"owner": {"login": "test-org"}
				},
				"sender": {"login": "testuser"}
			}`,
			wantErr: false,
			checks: func(t *testing.T, event *info.Event) {
				assert.Equal(t, event.TriggerComment, "")
			},
		},
		{
			name: "pull request with missing head repository",
			payload: `{
				"action": "opened",
				"number": 1,
				"pull_request": {
					"title": "Test PR",
					"head": {"ref": "feature", "sha": "abc123"},
					"base": {
						"ref": "main",
						"repo": {"html_url": "https://gitea.com/test-org/test-repo"}
					},
					"html_url": "https://gitea.com/test-org/test-repo/pulls/1"
				},
				"repository": {
					"name": "test-repo",
					"owner": {"login": "test-org"}
				},
				"sender": {"login": "testuser"}
			}`,
			wantErr: false,
			checks: func(t *testing.T, event *info.Event) {
				assert.Equal(t, event.HeadURL, "")
				assert.Equal(t, event.BaseURL, "https://gitea.com/test-org/test-repo")
				assert.Equal(t, event.SHAURL, "https://gitea.com/test-org/test-repo/pulls/1/commit/abc123")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := map[string]string{
				"X-Gitea-Event-Type": "pull_request",
			}
			// Determine event type based on test name
			switch {
			case strings.Contains(tt.name, "push"):
				headers["X-Gitea-Event-Type"] = "push"
			case strings.Contains(tt.name, "issue comment"):
				headers["X-Gitea-Event-Type"] = "issue_comment"
			}

			event, err := eventFromGitea([]byte(tt.payload), headers)
			if tt.wantErr {
				assert.Assert(t, err != nil)
				return
			}
			assert.NilError(t, err)
			assert.Assert(t, event != nil)
			if tt.checks != nil {
				tt.checks(t, event)
			}
		})
	}
}
