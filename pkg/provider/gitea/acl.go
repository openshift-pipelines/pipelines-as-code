package gitea

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"code.gitea.io/sdk/gitea"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/acl"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/policy"
	giteaStructs "github.com/openshift-pipelines/pipelines-as-code/pkg/provider/gitea/structs"
)

func (v *Provider) CheckPolicyAllowing(_ context.Context, event *info.Event, allowedTeams []string) (bool, string) {
	if event.Organization == event.Repository {
		return true, ""
	}
	// TODO: caching
	orgTeams, resp, err := v.Client.ListOrgTeams(event.Organization, gitea.ListTeamsOptions{})
	if resp.StatusCode == http.StatusNotFound {
		// we explicitly disallow the policy when there is no team on org
		return false, fmt.Sprintf("no teams on org %s", event.Organization)
	}
	if err != nil {
		// probably a 500 or another api error, no need to try again and again with other teams
		return false, fmt.Sprintf("error while getting org team, error: %s", err.Error())
	}
	for _, allowedTeam := range allowedTeams {
		for _, orgTeam := range orgTeams {
			if orgTeam.Name == allowedTeam {
				teamMember, _, err := v.Client.GetTeamMember(orgTeam.ID, event.Sender)
				if err != nil {
					v.Logger.Infof("error while getting team member: %s, error: %s", event.Sender, err.Error())
					continue
				}
				if teamMember.ID != 0 {
					return true, fmt.Sprintf("allowing user: %s as a member of the team: %s", event.Sender, orgTeam.Name)
				}
			}
		}
	}
	return false, fmt.Sprintf("user: %s is not a member of any of the allowed teams: %v", event.Sender, allowedTeams)
}

func (v *Provider) IsAllowed(ctx context.Context, event *info.Event, pac *info.PacOpts) (bool, error) {
	aclPolicy := policy.Policy{
		Settings: v.repoSettings,
		Event:    event,
		VCX:      v,
		Logger:   v.Logger,
	}

	// checkIfPolicyIsAllowing
	tType, _ := detectTriggerTypeFromPayload("", event.Event)
	policyRes, err := aclPolicy.IsAllowed(ctx, tType)
	switch policyRes {
	case policy.ResultAllowed:
		return true, nil
	case policy.ResultDisallowed:
		return false, err
	case policy.ResultNotSet:
		// showing as debug so we don't spill useless logs all the time in default info
		v.Logger.Debugf("policy check: policy is not set, checking for other conditions for sender: %s", event.Sender)
	}

	// Do most of the checks first, if user is a owner or in a organisation
	allowed, err := v.aclCheckAll(ctx, event)
	if err != nil {
		return false, fmt.Errorf("aclCheckAll: %w", err)
	}
	if allowed {
		return true, nil
	}

	// Finally try to parse comments
	return v.aclAllowedOkToTestFromAnOwner(ctx, event, pac)
}

// allowedOkToTestFromAnOwner Go over comments in a pull request and check
// if there is a /ok-to-test in there running an aclCheck again on the comment
// Sender if she is an OWNER and then allow it to run CI.
// TODO: pull out the github logic from there in an agnostic way.
func (v *Provider) aclAllowedOkToTestFromAnOwner(ctx context.Context, event *info.Event, pac *info.PacOpts) (bool, error) {
	revent := info.NewEvent()
	event.DeepCopyInto(revent)
	revent.EventType = ""
	revent.TriggerTarget = ""
	if revent.Event == nil {
		return false, nil
	}

	switch event := revent.Event.(type) {
	case *giteaStructs.IssueCommentPayload:
		// if we don't need to check old comments, then on issue comment we
		// need to check if comment have /ok-to-test and is from allowed user
		if !pac.RememberOKToTest {
			return v.aclAllowedOkToTestCurrentComment(ctx, revent, event.Comment.ID)
		}
		revent.URL = event.Issue.URL
	case *giteaStructs.PullRequestPayload:
		// if we don't need to check old comments, then on push event we don't need
		// to check anything for the non-allowed user
		if !pac.RememberOKToTest {
			return false, nil
		}
		revent.URL = event.PullRequest.HTMLURL
	default:
		return false, nil
	}

	comments, err := v.GetStringPullRequestComment(ctx, revent, acl.OKToTestCommentRegexp)
	if err != nil {
		return false, err
	}

	for _, comment := range comments {
		revent.Sender = comment.Poster.UserName
		allowed, err := v.aclCheckAll(ctx, revent)
		if err != nil {
			return false, err
		}
		if allowed {
			return true, nil
		}
	}
	return false, nil
}

// aclAllowedOkToTestCurrentEvent only check if this is issue comment event
// have /ok-to-test regex and sender is allowed.
func (v *Provider) aclAllowedOkToTestCurrentComment(ctx context.Context, revent *info.Event, id int64) (bool, error) {
	comment, _, err := v.Client.GetIssueComment(revent.Organization, revent.Repository, id)
	if err != nil {
		return false, err
	}
	if acl.MatchRegexp(acl.OKToTestCommentRegexp, comment.Body) {
		revent.Sender = comment.Poster.UserName
		allowed, err := v.aclCheckAll(ctx, revent)
		if err != nil {
			return false, err
		}
		if allowed {
			return true, nil
		}
	}
	return false, nil
}

// aclCheck check if we are allowed to run the pipeline on that PR
func (v *Provider) aclCheckAll(ctx context.Context, rev *info.Event) (bool, error) {
	if rev.Organization == rev.Sender {
		return true, nil
	}

	checkSenderRepoMembership, err := v.checkSenderRepoMembership(ctx, rev)
	if err != nil {
		return false, err
	}
	if checkSenderRepoMembership {
		return true, nil
	}

	// If we have a prow OWNERS file in the defaultBranch (ie: master) then
	// parse it in approvers and reviewers field and check if sender is in there.
	ownerContent, err := v.getFileFromDefaultBranch(ctx, "OWNERS", rev)
	if err != nil {
		if strings.Contains(err.Error(), "cannot find") {
			// no owner file, skipping
			return false, nil
		}
		return false, err
	}

	return acl.UserInOwnerFile(ownerContent, rev.Sender)
}

func (v *Provider) checkSenderRepoMembership(_ context.Context, runevent *info.Event) (bool, error) {
	ret, _, err := v.Client.IsCollaborator(runevent.Organization, runevent.Repository, runevent.Sender)
	return ret, err
}

// getFileFromDefaultBranch will get a file directly from the Default BaseBranch as
// configured in runinfo which is directly set in webhook by Github
func (v *Provider) getFileFromDefaultBranch(ctx context.Context, path string, runevent *info.Event) (string, error) {
	tektonyaml, err := v.GetFileInsideRepo(ctx, runevent, path, runevent.DefaultBranch)
	if err != nil {
		return "", fmt.Errorf("cannot find %s inside the %s branch: %w", path, runevent.DefaultBranch, err)
	}
	return tektonyaml, err
}

// GetStringPullRequestComment return the comment if we find a regexp in one of
// the comments text of a pull request
func (v *Provider) GetStringPullRequestComment(_ context.Context, runevent *info.Event, reg string) ([]*gitea.Comment, error) {
	var ret []*gitea.Comment
	prNumber, err := convertPullRequestURLtoNumber(runevent.URL)
	if err != nil {
		return nil, err
	}

	comments, _, err := v.Client.ListIssueComments(runevent.Organization, runevent.Repository, int64(prNumber), gitea.ListIssueCommentOptions{})
	if err != nil {
		return nil, err
	}
	for _, v := range comments {
		if acl.MatchRegexp(reg, v.Body) {
			ret = append(ret, v)
		}
	}
	return ret, nil
}
