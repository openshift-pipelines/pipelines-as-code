package api

import (
	"time"

	"github.com/cli/cli/v2/internal/ghrepo"
	"github.com/shurcooL/githubv4"
)

type PullRequestReviewState int

const (
	ReviewApprove PullRequestReviewState = iota
	ReviewRequestChanges
	ReviewComment
)

type PullRequestReviewInput struct {
	Body  string
	State PullRequestReviewState
}

type PullRequestReviews struct {
	Nodes    []PullRequestReview
	PageInfo struct {
		HasNextPage bool
		EndCursor   string
	}
	TotalCount int
}

type PullRequestReview struct {
	ID                  string         `json:"id"`
	Author              CommentAuthor  `json:"author"`
	AuthorAssociation   string         `json:"authorAssociation"`
	Body                string         `json:"body"`
	SubmittedAt         *time.Time     `json:"submittedAt"`
	IncludesCreatedEdit bool           `json:"includesCreatedEdit"`
	ReactionGroups      ReactionGroups `json:"reactionGroups"`
	State               string         `json:"state"`
	URL                 string         `json:"url,omitempty"`
	Commit              Commit         `json:"commit"`
}

func AddReview(client *Client, repo ghrepo.Interface, pr *PullRequest, input *PullRequestReviewInput) error {
	var mutation struct {
		AddPullRequestReview struct {
			ClientMutationID string
		} `graphql:"addPullRequestReview(input:$input)"`
	}

	state := githubv4.PullRequestReviewEventComment
	switch input.State {
	case ReviewApprove:
		state = githubv4.PullRequestReviewEventApprove
	case ReviewRequestChanges:
		state = githubv4.PullRequestReviewEventRequestChanges
	}

	body := githubv4.String(input.Body)
	variables := map[string]interface{}{
		"input": githubv4.AddPullRequestReviewInput{
			PullRequestID: pr.ID,
			Event:         &state,
			Body:          &body,
		},
	}

	return client.Mutate(repo.RepoHost(), "PullRequestReviewAdd", &mutation, variables)
}

func (prr PullRequestReview) Identifier() string {
	return prr.ID
}

func (prr PullRequestReview) AuthorLogin() string {
	return prr.Author.Login
}

func (prr PullRequestReview) Association() string {
	return prr.AuthorAssociation
}

func (prr PullRequestReview) Content() string {
	return prr.Body
}

func (prr PullRequestReview) Created() time.Time {
	if prr.SubmittedAt == nil {
		return time.Time{}
	}
	return *prr.SubmittedAt
}

func (prr PullRequestReview) HiddenReason() string {
	return ""
}

func (prr PullRequestReview) IsEdited() bool {
	return prr.IncludesCreatedEdit
}

func (prr PullRequestReview) IsHidden() bool {
	return false
}

func (prr PullRequestReview) Link() string {
	return prr.URL
}

func (prr PullRequestReview) Reactions() ReactionGroups {
	return prr.ReactionGroups
}

func (prr PullRequestReview) Status() string {
	return prr.State
}
