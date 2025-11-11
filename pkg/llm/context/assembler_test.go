package context

import (
	"strings"
	"testing"
	"time"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/test/logger"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestBuildCELContext(t *testing.T) {
	logger, _ := logger.GetLogger()
	run := &params.Run{}
	kinteract := &kubeinteraction.Interaction{}
	assembler := NewAssembler(run, kinteract, logger)

	tests := []struct {
		name           string
		event          *info.Event
		expectedFields map[string]any
		checkFields    []string
	}{
		{
			name: "all basic event fields",
			event: &info.Event{
				EventType:     "pull_request",
				TriggerTarget: triggertype.PullRequest,
				SHA:           "abc123",
				SHATitle:      "feat: add new feature",
				BaseBranch:    "main",
				HeadBranch:    "feature-branch",
				DefaultBranch: "main",
				Organization:  "my-org",
				Repository:    "my-repo",
				URL:           "https://github.com/my-org/my-repo",
				SHAURL:        "https://github.com/my-org/my-repo/commit/abc123",
				BaseURL:       "https://github.com/my-org/my-repo",
				HeadURL:       "https://github.com/my-org/my-repo/tree/feature-branch",
				Sender:        "user123",
			},
			expectedFields: map[string]any{
				"event_type":         "pull_request",
				"trigger_target":     "pull_request",
				"sha":                "abc123",
				"sha_title":          "feat: add new feature",
				"base_branch":        "main",
				"head_branch":        "feature-branch",
				"default_branch":     "main",
				"organization":       "my-org",
				"repository":         "my-repo",
				"url":                "https://github.com/my-org/my-repo",
				"sha_url":            "https://github.com/my-org/my-repo/commit/abc123",
				"base_url":           "https://github.com/my-org/my-repo",
				"head_url":           "https://github.com/my-org/my-repo/tree/feature-branch",
				"sender":             "user123",
				"target_pipelinerun": "",
			},
			checkFields: []string{
				"event_type", "trigger_target", "sha", "sha_title",
				"base_branch", "head_branch", "default_branch",
				"organization", "repository", "url", "sha_url",
				"base_url", "head_url", "sender", "target_pipelinerun",
			},
		},
		{
			name: "pull request specific fields",
			event: &info.Event{
				EventType:         "pull_request",
				PullRequestNumber: 42,
				PullRequestTitle:  "Add new feature",
				PullRequestLabel:  []string{"enhancement", "needs-review"},
			},
			expectedFields: map[string]any{
				"pull_request_number": 42,
				"pull_request_title":  "Add new feature",
				"pull_request_labels": []string{"enhancement", "needs-review"},
			},
			checkFields: []string{
				"pull_request_number",
				"pull_request_title",
				"pull_request_labels",
			},
		},
		{
			name: "trigger comment field",
			event: &info.Event{
				TriggerComment: "/test",
			},
			expectedFields: map[string]any{
				"trigger_comment": "/test",
			},
			checkFields: []string{"trigger_comment"},
		},
		{
			name: "incoming webhook target pipelinerun",
			event: &info.Event{
				TargetPipelineRun: "my-pipeline-run",
			},
			expectedFields: map[string]any{
				"target_pipelinerun": "my-pipeline-run",
			},
			checkFields: []string{"target_pipelinerun"},
		},
		{
			name: "push event without PR fields",
			event: &info.Event{
				EventType:     "push",
				TriggerTarget: triggertype.Push,
				SHA:           "def456",
				BaseBranch:    "main",
				HeadBranch:    "main",
			},
			expectedFields: map[string]any{
				"event_type":     "push",
				"trigger_target": "push",
				"sha":            "def456",
				"base_branch":    "main",
				"head_branch":    "main",
			},
			checkFields: []string{
				"event_type",
				"trigger_target",
				"sha",
				"base_branch",
				"head_branch",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := &tektonv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pr",
					Namespace: "test-ns",
				},
			}
			repo := &v1alpha1.Repository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-repo",
					Namespace: "test-ns",
				},
			}

			celContext, err := assembler.BuildCELContext(pr, tt.event, repo)
			assert.NilError(t, err)

			// Verify structure exists
			body, ok := celContext["body"].(map[string]any)
			assert.Assert(t, ok, "body should be a map")

			eventMap, ok := body["event"].(map[string]any)
			assert.Assert(t, ok, "body.event should be a map")

			// Check each expected field
			for _, field := range tt.checkFields {
				expectedValue, exists := tt.expectedFields[field]
				assert.Assert(t, exists, "expected field %s not found in test data", field)

				actualValue, exists := eventMap[field]
				assert.Assert(t, exists, "field %s should exist in event map", field)

				// For slices, use deep equal
				switch v := expectedValue.(type) {
				case []string:
					actualSlice, ok := actualValue.([]string)
					assert.Assert(t, ok, "field %s should be []string", field)
					assert.DeepEqual(t, actualSlice, v)
				default:
					assert.Equal(t, actualValue, expectedValue, "field %s has wrong value", field)
				}
			}
		})
	}
}

func TestBuildCELContext_ExcludedFields(t *testing.T) {
	logger, _ := logger.GetLogger()
	run := &params.Run{}
	kinteract := &kubeinteraction.Interaction{}
	assembler := NewAssembler(run, kinteract, logger)

	// Create event with fields that should be excluded
	event := &info.Event{
		EventType: "pull_request",
		Provider: &info.Provider{
			Token:         "secret-token",
			WebhookSecret: "webhook-secret",
			URL:           "https://api.github.com",
		},
		InstallationID:  12345,
		AccountID:       "bitbucket-account",
		GHEURL:          "https://ghe.example.com",
		CloneURL:        "https://bitbucket.example.com/scm/repo.git",
		SourceProjectID: 100,
		TargetProjectID: 200,
	}

	pr := &tektonv1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pr",
			Namespace: "test-ns",
		},
	}
	repo := &v1alpha1.Repository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-repo",
			Namespace: "test-ns",
		},
	}

	celContext, err := assembler.BuildCELContext(pr, event, repo)
	assert.NilError(t, err)

	body, ok := celContext["body"].(map[string]any)
	assert.Assert(t, ok)

	eventMap, ok := body["event"].(map[string]any)
	assert.Assert(t, ok)

	// Verify excluded fields are NOT present
	excludedFields := []string{
		"provider",
		"installation_id",
		"account_id",
		"ghe_url",
		"clone_url",
		"source_project_id",
		"target_project_id",
		"request",
		"state",
	}

	for _, field := range excludedFields {
		_, exists := eventMap[field]
		assert.Assert(t, !exists, "field %s should be excluded from CEL context", field)
	}
}

func TestBuildCELContext_NilEvent(t *testing.T) {
	logger, _ := logger.GetLogger()
	run := &params.Run{}
	kinteract := &kubeinteraction.Interaction{}
	assembler := NewAssembler(run, kinteract, logger)

	pr := &tektonv1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pr",
			Namespace: "test-ns",
		},
	}
	repo := &v1alpha1.Repository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-repo",
			Namespace: "test-ns",
		},
	}

	celContext, err := assembler.BuildCELContext(pr, nil, repo)
	assert.NilError(t, err)

	body, ok := celContext["body"].(map[string]any)
	assert.Assert(t, ok)

	// Event should not exist in the map
	_, exists := body["event"]
	assert.Assert(t, !exists, "event should not exist when nil event is passed")
}

func TestBuildCELContext_ConditionalFields(t *testing.T) {
	logger, _ := logger.GetLogger()
	run := &params.Run{}
	kinteract := &kubeinteraction.Interaction{}
	assembler := NewAssembler(run, kinteract, logger)

	pr := &tektonv1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pr",
			Namespace: "test-ns",
		},
	}
	repo := &v1alpha1.Repository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-repo",
			Namespace: "test-ns",
		},
	}

	t.Run("without pull request fields", func(t *testing.T) {
		event := &info.Event{
			EventType:         "push",
			PullRequestNumber: 0, // No PR
		}

		celContext, err := assembler.BuildCELContext(pr, event, repo)
		assert.NilError(t, err)

		body, ok := celContext["body"].(map[string]any)
		assert.Assert(t, ok)
		eventMap, ok := body["event"].(map[string]any)
		assert.Assert(t, ok)
		// PR fields should not exist
		_, exists := eventMap["pull_request_number"]
		assert.Assert(t, !exists, "pull_request_number should not exist for push events")

		_, exists = eventMap["pull_request_title"]
		assert.Assert(t, !exists, "pull_request_title should not exist for push events")

		_, exists = eventMap["pull_request_labels"]
		assert.Assert(t, !exists, "pull_request_labels should not exist for push events")
	})

	t.Run("without trigger comment", func(t *testing.T) {
		event := &info.Event{
			EventType:      "pull_request",
			TriggerComment: "", // No comment
		}

		celContext, err := assembler.BuildCELContext(pr, event, repo)
		assert.NilError(t, err)

		body, ok := celContext["body"].(map[string]any)
		assert.Assert(t, ok)
		eventMap, ok := body["event"].(map[string]any)
		assert.Assert(t, ok)

		// trigger_comment should not exist
		_, exists := eventMap["trigger_comment"]
		assert.Assert(t, !exists, "trigger_comment should not exist when empty")
	})
}

func TestBuildCommitContent(t *testing.T) {
	logger, _ := logger.GetLogger()
	run := &params.Run{}
	kinteract := &kubeinteraction.Interaction{}
	assembler := NewAssembler(run, kinteract, logger)
	ctx, _ := rtesting.SetupFakeContext(t)

	t.Run("basic commit fields without provider", func(t *testing.T) {
		event := &info.Event{
			SHA:      "abc123",
			SHATitle: "feat: add new feature",
		}

		commitData, err := assembler.buildCommitContent(ctx, event, nil)
		assert.NilError(t, err)

		// Should have basic fields
		assert.Equal(t, commitData["sha"], "abc123")
		assert.Equal(t, commitData["message"], "feat: add new feature")

		// Should not have extended fields
		_, hasURL := commitData["url"]
		assert.Assert(t, !hasURL, "url should not exist without provider")
	})

	t.Run("with full commit information after GetCommitInfo", func(t *testing.T) {
		event := &info.Event{
			SHA:               "abc123",
			SHATitle:          "feat: add new feature",
			SHAURL:            "https://github.com/org/repo/commit/abc123",
			SHAMessage:        "feat: add new feature\n\nThis is the detailed commit message explaining the changes.",
			SHAAuthorName:     "John Doe",
			SHAAuthorEmail:    "john@example.com", // Populated but excluded from LLM context
			SHACommitterName:  "GitHub",
			SHACommitterEmail: "noreply@github.com", // Populated but excluded from LLM context
		}

		commitData, err := assembler.buildCommitContent(ctx, event, nil)
		assert.NilError(t, err)

		// Basic fields
		assert.Equal(t, commitData["sha"], "abc123")
		assert.Equal(t, commitData["message"], "feat: add new feature")
		assert.Equal(t, commitData["url"], "https://github.com/org/repo/commit/abc123")

		// Full message
		fullMsg, ok := commitData["full_message"].(string)
		assert.Assert(t, ok, "full_message should be a string")
		assert.Assert(t, strings.Contains(fullMsg, "detailed commit message"))

		// Author information (name only, email excluded for privacy)
		author, ok := commitData["author"].(map[string]any)
		assert.Assert(t, ok, "author should be a map")
		assert.Equal(t, author["name"], "John Doe")
		_, hasEmail := author["email"]
		assert.Assert(t, !hasEmail, "email should be excluded for privacy/PII reasons")

		// Committer information (name only, email excluded for privacy)
		committer, ok := commitData["committer"].(map[string]any)
		assert.Assert(t, ok, "committer should be a map")
		assert.Equal(t, committer["name"], "GitHub")
		_, hasEmail = committer["email"]
		assert.Assert(t, !hasEmail, "email should be excluded for privacy/PII reasons")
	})

	t.Run("with author date and committer date", func(t *testing.T) {
		authorDate := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
		committerDate := time.Date(2024, 1, 15, 10, 31, 0, 0, time.UTC)

		event := &info.Event{
			SHA:              "abc123",
			SHATitle:         "fix: bug fix",
			SHAAuthorName:    "Jane Developer",
			SHAAuthorDate:    authorDate,
			SHACommitterName: "CI Bot",
			SHACommitterDate: committerDate,
		}

		commitData, err := assembler.buildCommitContent(ctx, event, nil)
		assert.NilError(t, err)

		author, ok := commitData["author"].(map[string]any)
		assert.Assert(t, ok)
		assert.DeepEqual(t, author["date"], authorDate)

		committer, ok := commitData["committer"].(map[string]any)
		assert.Assert(t, ok)
		assert.DeepEqual(t, committer["date"], committerDate)
	})

	t.Run("when full_message equals title, don't duplicate", func(t *testing.T) {
		event := &info.Event{
			SHA:        "abc123",
			SHATitle:   "fix: simple fix",
			SHAMessage: "fix: simple fix", // Same as title
		}

		commitData, err := assembler.buildCommitContent(ctx, event, nil)
		assert.NilError(t, err)

		// full_message should not be present since it's the same as title
		_, hasFull := commitData["full_message"]
		assert.Assert(t, !hasFull, "full_message should not exist when same as title")
	})

	t.Run("verify emails are always excluded even when present", func(t *testing.T) {
		event := &info.Event{
			SHA:               "abc123",
			SHATitle:          "feat: new feature",
			SHAAuthorName:     "John Doe",
			SHAAuthorEmail:    "john@example.com",   // Present but should be excluded
			SHACommitterEmail: "commit@example.com", // Present but should be excluded
		}

		commitData, err := assembler.buildCommitContent(ctx, event, nil)
		assert.NilError(t, err)

		// Email should never be included, even when populated
		author, ok := commitData["author"].(map[string]any)
		assert.Assert(t, ok, "author should exist")
		assert.Equal(t, author["name"], "John Doe")
		_, hasAuthorEmail := author["email"]
		assert.Assert(t, !hasAuthorEmail, "author email must be excluded for privacy")

		// Committer should not exist (no name provided)
		_, hasCommitter := commitData["committer"]
		assert.Assert(t, !hasCommitter, "committer should not exist without name or date")
	})

	t.Run("nil event", func(t *testing.T) {
		_, err := assembler.buildCommitContent(ctx, nil, nil)
		assert.ErrorContains(t, err, "event is nil")
	})
}
