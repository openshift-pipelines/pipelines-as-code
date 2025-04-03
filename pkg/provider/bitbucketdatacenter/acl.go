package bitbucketdatacenter

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	bbv1 "github.com/gfleury/go-bitbucket-v1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/acl"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
)

type activitiesTypes struct{ Values []*bbv1.Activity }

func (v *Provider) IsAllowed(ctx context.Context, event *info.Event) (bool, error) {
	allowed, err := v.checkMemberShip(ctx, event)
	if err != nil {
		return false, err
	}
	if allowed {
		return true, nil
	}

	// Check then from comment if there is a approved user that has done a /ok-to-test
	return v.checkOkToTestCommentFromApprovedMember(ctx, event)
}

// IsAllowedOwnersFile get the owner files (OWNERS, OWNERS_ALIASES) from main branch
// and check if we have explicitly allowed the user in there.
func (v *Provider) IsAllowedOwnersFile(ctx context.Context, event *info.Event) (bool, error) {
	ownerContent, err := v.GetFileInsideRepo(ctx, event, "OWNERS", event.DefaultBranch)
	if err != nil {
		return false, err
	}
	ownerAliasesContent, err := v.GetFileInsideRepo(ctx, event, "OWNERS_ALIASES", event.DefaultBranch)
	if err != nil {
		if !strings.Contains(err.Error(), "cannot find") {
			return false, err
		}
	}

	return acl.UserInOwnerFile(ownerContent, ownerAliasesContent, event.AccountID)
}

func (v *Provider) checkOkToTestCommentFromApprovedMember(ctx context.Context, event *info.Event) (bool, error) {
	allPages, err := paginate(func(nextPage int) (*bbv1.APIResponse, error) {
		localVarOptionals := map[string]any{
			"fromType": "COMMENT",
		}
		if nextPage > 0 {
			localVarOptionals["start"] = int(nextPage)
		}
		// will replace this API call with jenkins-x/go-scm after my PR on go-scm is merged
		// https://github.com/jenkins-x/go-scm/pull/494
		return v.Client.DefaultApi.GetActivities(v.projectKey, event.Repository, v.pullRequestNumber, localVarOptionals)
	})
	if err != nil {
		return false, err
	}

	for _, comment := range allPages {
		activities := &activitiesTypes{}
		cbyte, ok := comment.([]byte)
		if !ok {
			return false, fmt.Errorf("cannot convert comment to bytes")
		}
		err := json.Unmarshal(cbyte, activities)
		if err != nil {
			return false, err
		}
		for _, activity := range activities.Values {
			if acl.MatchRegexp(acl.OKToTestCommentRegexp, activity.Comment.Text) {
				commenterEvent := info.NewEvent()
				commenterEvent.Sender = activity.Comment.Author.Slug
				commenterEvent.AccountID = fmt.Sprintf("%d", activity.Comment.Author.ID)
				commenterEvent.Event = event.Event
				commenterEvent.BaseBranch = event.BaseBranch
				commenterEvent.HeadBranch = event.HeadBranch
				commenterEvent.Repository = event.Repository
				commenterEvent.Organization = v.projectKey
				commenterEvent.DefaultBranch = event.DefaultBranch
				allowed, err := v.checkMemberShip(ctx, commenterEvent)
				if err != nil {
					return false, err
				}
				if allowed {
					// TODO: show a log how come this has been allowed
					return true, nil
				}
			}
		}
	}
	return false, nil
}

func (v *Provider) checkMemberShip(ctx context.Context, event *info.Event) (bool, error) {
	// Get permissions from project
	allowed, _, err := v.ScmClient.Organizations.IsMember(ctx, event.Organization, event.Sender)
	if err != nil {
		return false, err
	}
	if allowed {
		return true, nil
	}

	orgAndRepo := fmt.Sprintf("%s/%s", event.Organization, event.Repository)
	// Get permissions from repo
	allowed, _, err = v.ScmClient.Repositories.IsCollaborator(ctx, orgAndRepo, event.Sender)
	if err != nil {
		return false, err
	}
	if allowed {
		return true, nil
	}

	// Check if sender is inside the Owner file
	// in the 'main' branch Silently ignore error, which should be fine it
	// probably means the OWNERS file is not created. If we had another error
	// (ie: like API) we probably would have hit it already.
	allowed, err = v.IsAllowedOwnersFile(ctx, event)
	if allowed {
		return true, err
	}

	return false, nil
}
