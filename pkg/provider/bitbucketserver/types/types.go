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
	Ref      Ref    `json:"ref"`
	FromHash string `json:"fromHash"`
	ToHash   string `json:"toHash"`
	RefID    string `json:"refId"`
	Type     string `json:"type"`
}

type Ref struct {
	ID        string `json:"id"`
	DisplayID string `json:"displayId"`
	Type      string `json:"type"`
}

type PushRequestEvent struct {
	EventKey   string                   `json:"eventKey"`
	Actor      bbv1.UserWithLinks       `json:"actor"`
	Repository bbv1.Repository          `json:"repository"`
	Changes    []PushRequestEventChange `json:"changes"`
	Commits    []bbv1.Commit            `json:"commits"`
	ToCommit   ToCommit                 `json:"toCommit"`
}

type ToCommit struct {
	bbv1.Commit
	Parents []bbv1.Commit `json:"parents"` // bbv1.Commit also has Parents field, but its Parents has only two fields while actual payload has more.
}
