package azuredevops

import (
	"context"
	"fmt"
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/acl"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/opscomments"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
)

// ToDo: implement this function.
func (v *Provider) CheckPolicyAllowing(context.Context, *info.Event, []string) (bool, string) {
	return false, ""
}

func (v *Provider) IsAllowed(ctx context.Context, event *info.Event) (bool, error) {

	allowed, _ := v.checkMembership(ctx, event)
	if allowed {
		return true, nil
	}

	// Try to parse the comment from an owner who has issues a /ok-to-test
	ownerAllowed, err := v.aclAllowedOkToTestFromAnOwner(ctx, event)
	if err != nil {
		return false, err
	}
	if ownerAllowed {
		return true, nil
	}
	return false, nil
}

func (v *Provider) aclAllowedOkToTestFromAnOwner(ctx context.Context, event *info.Event) (bool, error) {
	if event.EventType == opscomments.OkToTestCommentEventType.String() {
		allowed, _ := v.checkMembership(ctx, event)
		if allowed {
			return true, nil
		}
	}
	return false, nil
}

func (v *Provider) checkMembership(ctx context.Context, event *info.Event) (bool, error) {
	teams, err := v.CoreClient.GetTeams(ctx, core.GetTeamsArgs{
		ProjectId: &event.ProjectID,
	})

	if err != nil {
		return false, err
	}

	// Check if the PR author is a member of any team
	for _, team := range *teams {
		if team.Id == nil {
			continue
		}
		teamIdStr := team.Id.String()
		members, err := v.CoreClient.GetTeamMembersWithExtendedProperties(ctx, core.GetTeamMembersWithExtendedPropertiesArgs{
			ProjectId: &event.ProjectID,
			TeamId:    &teamIdStr,
		})
		if err != nil {
			continue
		}

		for _, member := range *members {
			if *member.Identity.Id == event.Sender {
				return true, nil
			}
		}
	}
	return v.IsAllowedOwnersFile(ctx, event)
}

func (v *Provider) IsAllowedOwnersFile(ctx context.Context, event *info.Event) (bool, error) {
	ownerContent, err := v.getFileFromDefaultBranch(ctx, "OWNERS", event)
	if err != nil {
		if strings.Contains(err.Error(), "cannot find") {
			// no owner file, skipping
			return false, nil
		}
		return false, err
	}

	return acl.UserInOwnerFile(ownerContent, event.Sender)
}

func (v *Provider) getFileFromDefaultBranch(ctx context.Context, path string, runevent *info.Event) (string, error) {
	owner, err := v.GetFileInsideRepo(ctx, runevent, path, runevent.DefaultBranch)
	if err != nil {
		return "", fmt.Errorf("cannot find %s inside the %s branch: %w", path, runevent.DefaultBranch, err)
	}
	return owner, err
}
