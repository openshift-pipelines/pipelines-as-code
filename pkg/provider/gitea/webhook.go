package gitea

import (
	"encoding/json"
	"fmt"

	gitea "code.gitea.io/gitea/modules/structs"
)

// whEventType represents a Gitea webhook event
type whEventType string

// List of supported events
//
// Find them in Gitea's source at /models/webhook.go as HookEventType.
// To correlate with when each of these trigger, see the Trigger On -> Custom Events options
// when editing a repo's webhook in a Gitea project. Those descriptions are helpful.
const (
	EventTypeCreate              whEventType = "create"
	EventTypeDelete              whEventType = "delete"
	EventTypeFork                whEventType = "fork"
	EventTypePush                whEventType = "push"
	EventTypeIssues              whEventType = "issues"
	EventTypeIssueComment        whEventType = "issue_comment"
	EventTypeRepository          whEventType = "repository"
	EventTypeRelease             whEventType = "release"
	EventTypePullRequest         whEventType = "pull_request"
	EventTypePullRequestApproved whEventType = "pull_request_approved"
	EventTypePullRequestRejected whEventType = "pull_request_rejected"
	EventTypePullRequestComment  whEventType = "pull_request_comment"
	EventTypePullRequestSync     whEventType = "pull_request_sync"
)

func parseWebhook(eventType whEventType, payload []byte) (event interface{}, err error) {
	switch eventType {
	case EventTypePush:
		event = &gitea.PushPayload{}
	case EventTypeCreate:
		event = &gitea.CreatePayload{}
	case EventTypeDelete:
		event = &gitea.DeletePayload{}
	case EventTypeFork:
		event = &gitea.ForkPayload{}
	case EventTypeIssues:
		event = &gitea.IssuePayload{}
	case EventTypeIssueComment:
		event = &gitea.IssueCommentPayload{}
	case EventTypeRepository:
		event = &gitea.RepositoryPayload{}
	case EventTypeRelease:
		event = &gitea.ReleasePayload{}
	case EventTypePullRequestComment:
		event = &gitea.IssueCommentPayload{}
	case EventTypePullRequest, EventTypePullRequestApproved, EventTypePullRequestSync, EventTypePullRequestRejected:
		event = &gitea.PullRequestPayload{}
	default:
		return nil, fmt.Errorf("unexpected event type: %s", eventType)
	}

	if err := json.Unmarshal(payload, event); err != nil {
		return nil, err
	}

	return event, nil
}
