package gitea

import (
	"encoding/json"
	"fmt"

	giteaStructs "code.gitea.io/gitea/modules/structs"
)

// whEventType represents a Gitea webhook event.
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
	EventTypePullRequestLabel    whEventType = "pull_request_label"
	EventTypePullRequestComment  whEventType = "pull_request_comment"
	EventTypePullRequestSync     whEventType = "pull_request_sync"
)

func parseWebhook(eventType whEventType, payload []byte) (event any, err error) {
	switch eventType {
	case EventTypePush:
		event = &giteaStructs.PushPayload{}
	case EventTypeCreate:
		event = &giteaStructs.CreatePayload{}
	case EventTypeDelete:
		event = &giteaStructs.DeletePayload{}
	case EventTypeFork:
		event = &giteaStructs.ForkPayload{}
	case EventTypeIssues:
		event = &giteaStructs.IssuePayload{}
	case EventTypeIssueComment:
		event = &giteaStructs.IssueCommentPayload{}
	case EventTypeRepository:
		event = &giteaStructs.RepositoryPayload{}
	case EventTypeRelease:
		event = &giteaStructs.ReleasePayload{}
	case EventTypePullRequestComment:
		event = &giteaStructs.IssueCommentPayload{}
	case EventTypePullRequest, EventTypePullRequestApproved, EventTypePullRequestSync, EventTypePullRequestRejected, EventTypePullRequestLabel:
		event = &giteaStructs.PullRequestPayload{}
	default:
		return nil, fmt.Errorf("unexpected event type: %s", eventType)
	}

	if err := json.Unmarshal(payload, event); err != nil {
		return nil, err
	}

	return event, nil
}
