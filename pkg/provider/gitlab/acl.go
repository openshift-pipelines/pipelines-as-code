package gitlab

import (
	"context"
	"fmt"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/acl"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/xanzy/go-gitlab"
)

// get the owner file from main branch and check if we are allowing there
func (v *Provider) isAllowedFromOwnerFile(event *info.Event) bool {
	ownerContent, _ := v.getObject("OWNERS", event.DefaultBranch, v.targetProjectID)
	if string(ownerContent) == "" {
		return false
	}
	allowed, _ := acl.UserInOwnerFile(string(ownerContent), event.Sender)
	return allowed
}

func (v *Provider) checkMembership(event *info.Event, userid int) bool {
	var member *gitlab.ProjectMember
	var err error
	if event.Settings != nil && event.Settings.OnlyTrustsUsersFromRepository {
		v.Logger.Infof("Checking membership for user %d in project %d scopped to the repository due of the only_trusts_users_from_repository setting", userid, v.targetProjectID)
		member, _, err = v.Client.ProjectMembers.GetProjectMember(v.targetProjectID, userid)
	} else {
		v.Logger.Infof("Checking membership for user %d and inherited project %d and group", userid, v.targetProjectID)
		member, _, err = v.Client.ProjectMembers.GetInheritedProjectMember(v.targetProjectID, userid)
	}
	if err == nil && member.ID == userid {
		return true
	}

	return v.isAllowedFromOwnerFile(event)
}

func (v *Provider) checkOkToTestCommentFromApprovedMember(event *info.Event, page int) (bool, error) {
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
			if v.checkMembership(commenterEvent, topthread.Author.ID) {
				return true, nil
			}
		}
	}

	if nextPage != 0 {
		return v.checkOkToTestCommentFromApprovedMember(event, nextPage)
	}

	return false, nil
}

func (v *Provider) IsAllowed(_ context.Context, event *info.Event) (bool, error) {
	if v.Client == nil {
		return false, fmt.Errorf("no github client has been initiliazed, " +
			"exiting... (hint: did you forget setting a secret on your repo?)")
	}
	if v.checkMembership(event, v.userID) {
		return true, nil
	}

	return v.checkOkToTestCommentFromApprovedMember(event, 1)
}
