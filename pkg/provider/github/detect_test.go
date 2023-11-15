package github

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/go-github/v56/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/test/logger"
	"gotest.tools/v3/assert"
)

func TestProvider_Detect(t *testing.T) {
	tests := []struct {
		name          string
		wantErrString string
		isGH          bool
		processReq    bool
		event         interface{}
		eventType     string
		wantReason    string
	}{
		{
			name:       "not a github Event",
			eventType:  "",
			isGH:       false,
			processReq: false,
		},
		{
			name:          "invalid github Event",
			eventType:     "validator",
			wantErrString: "unknown X-Github-Event in message: validator",
			isGH:          false,
			processReq:    false,
		},
		{
			name: "valid check suite Event",
			event: github.CheckSuiteEvent{
				Action: github.String("rerequested"),
				CheckSuite: &github.CheckSuite{
					ID: github.Int64(123),
				},
			},
			eventType:  "check_suite",
			isGH:       true,
			processReq: true,
		},
		{
			name: "valid check run Event",
			event: github.CheckRunEvent{
				Action: github.String("rerequested"),
				CheckRun: &github.CheckRun{
					ID: github.Int64(123),
				},
			},
			eventType:  "check_run",
			isGH:       true,
			processReq: true,
		},
		{
			name: "unsupported Event",
			event: github.CommitCommentEvent{
				Action: github.String("something"),
			},
			eventType:  "release",
			wantReason: "event \"release\" is not supported",
			isGH:       true,
			processReq: false,
		},
		{
			name: "invalid commit_comment Event",
			event: github.CommitCommentEvent{
				Action: github.String("something"),
			},
			eventType:  "commit_comment",
			isGH:       true,
			processReq: false,
		},
		{
			name: "invalid check run Event",
			event: github.CheckRunEvent{
				Action: github.String("not rerequested"),
			},
			eventType:  "check_run",
			isGH:       true,
			processReq: false,
		},
		{
			name: "invalid issue comment Event",
			event: github.IssueCommentEvent{
				Action: github.String("deleted"),
			},
			eventType:  "issue_comment",
			isGH:       true,
			processReq: false,
		},
		{
			name: "issue comment Event with no valid comment",
			event: github.IssueCommentEvent{
				Action: github.String("created"),
				Issue: &github.Issue{
					PullRequestLinks: &github.PullRequestLinks{
						URL: github.String("url"),
					},
					State: github.String("open"),
				},
				Installation: &github.Installation{
					ID: github.Int64(123),
				},
				Comment: &github.IssueComment{Body: github.String("abc")},
			},
			eventType:  "issue_comment",
			isGH:       true,
			processReq: false,
		},
		{
			name: "issue comment Event with ok-to-test comment",
			event: github.IssueCommentEvent{
				Action: github.String("created"),
				Issue: &github.Issue{
					PullRequestLinks: &github.PullRequestLinks{
						URL: github.String("url"),
					},
					State: github.String("open"),
				},
				Installation: &github.Installation{
					ID: github.Int64(123),
				},
				Comment: &github.IssueComment{Body: github.String("/ok-to-test")},
			},
			eventType:  "issue_comment",
			isGH:       true,
			processReq: true,
		},
		{
			name: "issue comment Event with ok-to-test and some string",
			event: github.IssueCommentEvent{
				Action: github.String("created"),
				Issue: &github.Issue{
					PullRequestLinks: &github.PullRequestLinks{
						URL: github.String("url"),
					},
					State: github.String("open"),
				},
				Installation: &github.Installation{
					ID: github.Int64(123),
				},
				Comment: &github.IssueComment{Body: github.String("/ok-to-test \n let me in :)")},
			},
			eventType:  "issue_comment",
			isGH:       true,
			processReq: true,
		},
		{
			name: "issue comment Event with retest",
			event: github.IssueCommentEvent{
				Action: github.String("created"),
				Issue: &github.Issue{
					PullRequestLinks: &github.PullRequestLinks{
						URL: github.String("url"),
					},
					State: github.String("open"),
				},
				Installation: &github.Installation{
					ID: github.Int64(123),
				},
				Comment: &github.IssueComment{Body: github.String("/retest")},
			},
			eventType:  "issue_comment",
			isGH:       true,
			processReq: true,
		},
		{
			name: "issue comment Event with retest with some string",
			event: github.IssueCommentEvent{
				Action: github.String("created"),
				Issue: &github.Issue{
					PullRequestLinks: &github.PullRequestLinks{
						URL: github.String("url"),
					},
					State: github.String("open"),
				},
				Installation: &github.Installation{
					ID: github.Int64(123),
				},
				Comment: &github.IssueComment{Body: github.String("/retest \n will you retest?")},
			},
			eventType:  "issue_comment",
			isGH:       true,
			processReq: true,
		},
		{
			name: "push event",
			event: github.PushEvent{
				Pusher: &github.User{ID: github.Int64(11)},
			},
			eventType:  "push",
			isGH:       true,
			processReq: true,
		},
		{
			name: "pull request event",
			event: github.PullRequestEvent{
				Action: github.String("opened"),
			},
			eventType:  "pull_request",
			isGH:       true,
			processReq: true,
		},
		{
			name: "pull request event not supported action",
			event: github.PullRequestEvent{
				Action: github.String("deleted"),
			},
			eventType:  "pull_request",
			isGH:       true,
			processReq: false,
		},
		{
			name: "issue comment event with cancel comment",
			event: github.IssueCommentEvent{
				Action: github.String("created"),
				Issue: &github.Issue{
					PullRequestLinks: &github.PullRequestLinks{
						URL: github.String("url"),
					},
					State: github.String("open"),
				},
				Installation: &github.Installation{
					ID: github.Int64(123),
				},
				Comment: &github.IssueComment{Body: github.String("/cancel")},
			},
			eventType:  "issue_comment",
			isGH:       true,
			processReq: true,
		},
		{
			name: "issue comment Event with cancel comment ",
			event: github.IssueCommentEvent{
				Action: github.String("created"),
				Issue: &github.Issue{
					PullRequestLinks: &github.PullRequestLinks{
						URL: github.String("url"),
					},
					State: github.String("open"),
				},
				Installation: &github.Installation{
					ID: github.Int64(123),
				},
				Comment: &github.IssueComment{Body: github.String("/cancel dummy")},
			},
			eventType:  "issue_comment",
			isGH:       true,
			processReq: true,
		},
		{
			name: "commit comment event with cancel comment",
			event: github.CommitCommentEvent{
				Action: github.String("created"),
				Installation: &github.Installation{
					ID: github.Int64(123),
				},
				Comment: &github.RepositoryComment{Body: github.String("/cancel")},
			},
			eventType:  "commit_comment",
			isGH:       true,
			processReq: true,
		},
		{
			name: "commit comment Event with retest",
			event: github.CommitCommentEvent{
				Action: github.String("created"),
				Installation: &github.Installation{
					ID: github.Int64(123),
				},
				Comment: &github.RepositoryComment{Body: github.String("/retest")},
			},
			eventType:  "commit_comment",
			isGH:       true,
			processReq: true,
		},
		{
			name: "commit comment Event with test",
			event: github.CommitCommentEvent{
				Action: github.String("created"),
				Installation: &github.Installation{
					ID: github.Int64(123),
				},
				Comment: &github.RepositoryComment{Body: github.String("/test")},
			},
			eventType:  "commit_comment",
			isGH:       true,
			processReq: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gprovider := Provider{}
			logger, _ := logger.GetLogger()
			jeez, err := json.Marshal(tt.event)
			if err != nil {
				assert.NilError(t, err)
			}

			header := http.Header{}
			header.Set("X-GitHub-Event", tt.eventType)

			req := &http.Request{Header: header}
			isGh, processReq, _, reason, err := gprovider.Detect(req, string(jeez), logger)
			if tt.wantErrString != "" {
				assert.ErrorContains(t, err, tt.wantErrString)
				return
			}
			if tt.wantReason != "" {
				assert.Assert(t, strings.Contains(reason, tt.wantReason), reason, tt.wantReason)
				return
			}
			assert.NilError(t, err)
			assert.Equal(t, tt.isGH, isGh)
			assert.Equal(t, tt.processReq, processReq)
		})
	}
}
