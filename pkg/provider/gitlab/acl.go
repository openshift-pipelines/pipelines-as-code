package gitlab

import (
	"context"
	"fmt"
	"net/http"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/acl"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	gitlab "gitlab.com/gitlab-org/api/client-go"
)

// IsAllowedOwnersFile get the owner files (OWNERS, OWNERS_ALIASES) from main branch
// and check if we have explicitly allowed the user in there.
func (v *Provider) IsAllowedOwnersFile(_ context.Context, event *info.Event) (bool, error) {
	ownerContent, _, _ := v.getObject("OWNERS", event.DefaultBranch, v.targetProjectID)
	if string(ownerContent) == "" {
		return false, nil
	}
	// OWNERS_ALIASES file existence is not required, if we get "not found" continue
	ownerAliasesContent, resp, err := v.getObject("OWNERS_ALIASES", event.DefaultBranch, v.targetProjectID)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		return false, err
	}
	allowed, _ := acl.UserInOwnerFile(string(ownerContent), string(ownerAliasesContent), event.Sender)
	return allowed, nil
}

func (v *Provider) checkMembership(ctx context.Context, event *info.Event, userid int) bool {
	member, _, err := v.Client().ProjectMembers.GetInheritedProjectMember(v.targetProjectID, userid)
	if err == nil && member.ID != 0 && member.ID == userid {
		return true
	}

	isAllowed, _ := v.IsAllowedOwnersFile(ctx, event)
	return isAllowed
}

func (v *Provider) checkOkToTestCommentFromApprovedMember(ctx context.Context, event *info.Event, page int) (bool, error) {
	var nextPage int
	opt := &gitlab.ListMergeRequestDiscussionsOptions{Page: page}
	discussions, resp, err := v.Client().Discussions.ListMergeRequestDiscussions(v.targetProjectID, event.PullRequestNumber, opt)
	if err != nil || len(discussions) == 0 {
		return false, err
	}
	if resp.NextPage != 0 {
		nextPage = resp.NextPage
	}

	for _, discussion := range discussions {
		// Iterate through every note in the discussion thread and evaluate them.
		// If a note contains an OK-to-test command, verify the commenter's permission
		// (either project membership or presence in OWNERS/OWNERS_ALIASES).
		for _, note := range discussion.Notes {
			if acl.MatchRegexp(acl.OKToTestCommentRegexp, note.Body) {
				commenterEvent := info.NewEvent()
				commenterEvent.Event = event.Event
				commenterEvent.Sender = note.Author.Username
				commenterEvent.BaseBranch = event.BaseBranch
				commenterEvent.HeadBranch = event.HeadBranch
				commenterEvent.DefaultBranch = event.DefaultBranch
				// We could add caching for membership checks in the future.
				if v.checkMembership(ctx, commenterEvent, note.Author.ID) {
					return true, nil
				}
			}
		}
	}

	if nextPage != 0 {
		return v.checkOkToTestCommentFromApprovedMember(ctx, event, nextPage)
	}

	return false, nil
}

func (v *Provider) IsAllowed(ctx context.Context, event *info.Event) (bool, error) {
	if v.gitlabClient == nil {
		return false, fmt.Errorf("no github client has been initialized, " +
			"exiting... (hint: did you forget setting a secret on your repo?)")
	}
	if v.checkMembership(ctx, event, v.userID) {
		return true, nil
	}

	return v.checkOkToTestCommentFromApprovedMember(ctx, event, 1)
}
