package types

import (
	bbv1 "github.com/gfleury/go-bitbucket-v1"
)

type EventActor struct {
	ID   int
	Name string
}

type PullRequestEvent struct {
	Actor       bbv1.UserWithLinks `json:"actor"`
	PullRequest bbv1.PullRequest   `json:"pullRequest"`
	EventKey    string             `json:"eventKey"`

	// Comment should be used when event is `pr:comment:added` or `pr:comment:edited`.
	Comment bbv1.ActivityComment `json:"comment"`

	// CommentParentID and PreviousComment should be used when event is `pr:comment:edited`.
	CommentParentID string `json:"commentParentId"`
	PreviousComment string `json:"previousComment"`
}

type PushRequestEventChange struct {
	ToHash string `json:"toHash"`
	RefID  string `json:"refId"`
}

type PushRequestEvent struct {
	Actor      bbv1.UserWithLinks       `json:"actor"`
	Repository bbv1.Repository          `json:"repository"`
	Changes    []PushRequestEventChange `json:"changes"`
	Commits    []bbv1.Commit            `json:"commits"`
}
