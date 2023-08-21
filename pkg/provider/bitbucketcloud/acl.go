package bitbucketcloud

import (
	"context"
	"strconv"

	"github.com/ktrysmt/go-bitbucket"
	"github.com/mitchellh/mapstructure"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/acl"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/bitbucketcloud/types"
)

func (v *Provider) IsAllowed(_ context.Context, event *info.Event, _ *info.PacOpts) (bool, error) {
	// Check first if the user is in the owner file or part of the workspace
	allowed, err := v.checkMember(event)
	if err != nil {
		return false, err
	}
	if allowed {
		return true, nil
	}

	// Check then from comment if there is a approved user that has done a /ok-to-test
	return v.checkOkToTestCommentFromApprovedMember(event)
}

func (v *Provider) isWorkspaceMember(event *info.Event) (bool, error) {
	members, err := v.Client.Workspaces.Members(event.Organization)
	if err != nil {
		return false, err
	}

	for _, member := range members.Members {
		if member.AccountId == event.AccountID {
			return true, nil
		}
	}
	return false, nil
}

// get the owner file from main branch and check if we are allowing there
func (v *Provider) isAllowedFromOwnerFile(event *info.Event) (bool, error) {
	ownerContent, err := v.GetFileInsideRepo(context.TODO(), event, "OWNERS", event.DefaultBranch)
	if err != nil {
		return false, err
	}

	return acl.UserInOwnerFile(ownerContent, event.AccountID)
}

func (v *Provider) checkMember(event *info.Event) (bool, error) {
	// If sender is a member that can write to the workspace then allow it.
	allowed, err := v.isWorkspaceMember(event)
	if err != nil {
		return false, err
	} else if allowed {
		return true, err
	}

	// Check if sender (which in bitbucket-cloud mean the accountID) is inside the Owner file
	// in the 'main' branch Silently ignore error, which should be fine it
	// probably means the OWNERS file is not created. If we had another error
	// (ie: like API) we probably would have hit it already.
	allowed, _ = v.isAllowedFromOwnerFile(event)
	if allowed {
		return true, err
	}

	return false, nil
}

func (v *Provider) checkOkToTestCommentFromApprovedMember(event *info.Event) (bool, error) {
	commentsIntf, err := v.Client.Repositories.PullRequests.GetComments(&bitbucket.PullRequestsOptions{
		Owner:    event.Organization,
		RepoSlug: event.Repository,
		ID:       strconv.Itoa(event.PullRequestNumber),
	})
	if err != nil {
		return false, err
	}
	comments := &types.Comments{}
	err = mapstructure.Decode(commentsIntf, comments)
	if err != nil {
		return false, err
	}
	for _, comment := range comments.Values {
		if acl.MatchRegexp(acl.OKToTestCommentRegexp, comment.Content.Raw) {
			commenterEvent := info.NewEvent()
			commenterEvent.Event = event.Event
			commenterEvent.Sender = comment.User.Nickname
			commenterEvent.AccountID = comment.User.AccountID
			commenterEvent.BaseBranch = event.BaseBranch
			commenterEvent.HeadBranch = event.HeadBranch
			commenterEvent.Repository = event.Repository
			commenterEvent.Organization = event.Organization
			commenterEvent.DefaultBranch = event.DefaultBranch
			allowed, err := v.checkMember(commenterEvent)
			if err != nil {
				return false, err
			}
			if allowed {
				return true, nil
			}
		}
	}

	return false, nil
}
