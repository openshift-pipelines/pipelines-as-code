package gitlab

import (
	"context"
	"fmt"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/acl"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/xanzy/go-gitlab"
)

// IsAllowedOwnersFile get the owner file from main branch and check if we have
// explicitly allowed the user in there.
func (v *Provider) IsAllowedOwnersFile(_ context.Context, event *info.Event) (bool, error) {
	ownerContent, _ := v.getObject("OWNERS", event.DefaultBranch, v.targetProjectID)
	if string(ownerContent) == "" {
		return false, nil
	}
	allowed, _ := acl.UserInOwnerFile(string(ownerContent), event.Sender)
	return allowed, nil
}

func (v *Provider) checkMembership(ctx context.Context, event *info.Event, userid int) bool {
	member, _, err := v.Client.ProjectMembers.GetInheritedProjectMember(v.targetProjectID, userid)
	if err == nil && member.ID == userid {
		return true
	}

	isAllowed, _ := v.IsAllowedOwnersFile(ctx, event)
	return isAllowed
}

func (v *Provider) checkOkToTestCommentFromApprovedMember(ctx context.Context, event *info.Event, page int) (bool, error) {
	var nextPage int
	opt := &gitlab.ListMergeRequestDiscussionsOptions{Page: page}
	discussions, resp, err := v.Client.Discussions.ListMergeRequestDiscussions(v.targetProjectID, event.PullRequestNumber, opt)
	if err != nil {
		return false, err
	}
	if resp.NextPage != 0 {
		nextPage = resp.NextPage
	}

	for _, comment := range discussions {
		// TODO: maybe we do threads in the future but for now we just check the top thread for ops related comments
		topthread := comment.Notes[0]
		if acl.MatchRegexp(acl.OKToTestCommentRegexp, topthread.Body) {
			commenterEvent := info.NewEvent()
			commenterEvent.Event = event.Event
			commenterEvent.Sender = topthread.Author.Username
			commenterEvent.BaseBranch = event.BaseBranch
			commenterEvent.HeadBranch = event.HeadBranch
			commenterEvent.DefaultBranch = event.DefaultBranch
			// TODO: we could probably do with caching when checking all issues?
			if v.checkMembership(ctx, commenterEvent, topthread.Author.ID) {
				return true, nil
			}
		}
	}

	if nextPage != 0 {
		return v.checkOkToTestCommentFromApprovedMember(ctx, event, nextPage)
	}

	return false, nil
}

func (v *Provider) IsAllowed(ctx context.Context, event *info.Event) (bool, error) {
	if v.Client == nil {
		return false, fmt.Errorf("no github client has been initialized, " +
			"exiting... (hint: did you forget setting a secret on your repo?)")
	}
	if v.checkMembership(ctx, event, v.userID) {
		return true, nil
	}

	return v.checkOkToTestCommentFromApprovedMember(ctx, event, 1)
}
