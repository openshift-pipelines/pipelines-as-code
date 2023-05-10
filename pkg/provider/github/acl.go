package github

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/go-github/v52/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/acl"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
)

func (v *Provider) IsAllowed(ctx context.Context, event *info.Event) (bool, error) {
	// Do most of the checks first, if user is a owner or in a organisation
	allowed, err := v.aclCheckAll(ctx, event)
	if err != nil {
		return false, err
	}
	if allowed {
		return true, nil
	}

	// Finally try to parse all comments
	return v.aclAllowedOkToTestFromAnOwner(ctx, event)
}

// allowedOkToTestFromAnOwner Go over every comments in a pull request and check
// if there is a /ok-to-test in there running an aclCheck again on the comment
// Sender if she is an OWNER and then allow it to run CI.
// TODO: pull out the github logic from there in an agnostic way.
func (v *Provider) aclAllowedOkToTestFromAnOwner(ctx context.Context, event *info.Event) (bool, error) {
	revent := info.NewEvent()
	event.DeepCopyInto(revent)
	revent.EventType = ""
	revent.TriggerTarget = ""
	if revent.Event == nil {
		return false, nil
	}

	switch event := revent.Event.(type) {
	case *github.IssueCommentEvent:
		revent.URL = event.Issue.GetPullRequestLinks().GetHTMLURL()
	case *github.PullRequestEvent:
		revent.URL = event.GetPullRequest().GetHTMLURL()
	default:
		return false, nil
	}

	comments, err := v.GetStringPullRequestComment(ctx, revent, acl.OKToTestCommentRegexp)
	if err != nil {
		return false, err
	}

	for _, comment := range comments {
		revent.Sender = comment.User.GetLogin()
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

	// If the user who has submitted the PR is not a owner or public member or Collaborator or not there in OWNERS file
	// but has permission to push to branches then allow the CI to be run.
	// This can only happen with GithubApp and Bots.
	// Ex: dependabot, bots
	if rev.PullRequestNumber != 0 {
		isSameCloneURL, err := v.checkPullRequestForSameURL(ctx, rev)
		if err != nil {
			return false, err
		}
		if isSameCloneURL {
			return true, nil
		}
	}

	// If the user who has submitted the pr is a owner on the repo then allows
	// the CI to be run.
	isUserMemberRepo, err := v.checkSenderOrgMembership(ctx, rev)
	if err != nil {
		return false, err
	}
	if isUserMemberRepo {
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

// checkPullRequestForSameURL checks If PullRequests are for same clone URL and different branches
// means if the user has access to create a branch in the repository without forking or having any permissions then PAC should allow to run CI.
//
//	ex: dependabot, *[bot] etc...
func (v *Provider) checkPullRequestForSameURL(ctx context.Context, runevent *info.Event) (bool, error) {
	pr, resp, err := v.Client.PullRequests.Get(ctx, runevent.Organization, runevent.Repository, runevent.PullRequestNumber)
	if err != nil {
		return false, err
	}
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		return false, nil
	}

	if pr.GetHead().GetRepo().GetCloneURL() == pr.GetBase().GetRepo().GetCloneURL() && pr.GetHead().GetRef() != pr.GetBase().GetRef() {
		return true, nil
	}

	return false, nil
}

// checkSenderOrgMembership Get sender user's organization. We can
// only get the one that the user sets as public 🤷
func (v *Provider) checkSenderOrgMembership(ctx context.Context, runevent *info.Event) (bool, error) {
	users, resp, err := v.Client.Organizations.ListMembers(ctx, runevent.Organization,
		&github.ListMembersOptions{
			PublicOnly: true, // We can't list private member in a org
		})
	// If we are 404 it means we are checking a repo owner and not a org so let's bail out with grace
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		return false, nil
	}

	if err != nil {
		return false, err
	}
	for _, v := range users {
		if v.GetLogin() == runevent.Sender {
			return true, nil
		}
	}

	return false, nil
}

// checkSenderRepoMembership check if user is allowed to run CI
func (v *Provider) checkSenderRepoMembership(ctx context.Context, runevent *info.Event) (bool, error) {
	isCollab, _, err := v.Client.Repositories.IsCollaborator(ctx,
		runevent.Organization,
		runevent.Repository,
		runevent.Sender)

	return isCollab, err
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
func (v *Provider) GetStringPullRequestComment(ctx context.Context, runevent *info.Event, reg string) ([]*github.IssueComment, error) {
	var ret []*github.IssueComment
	prNumber, err := convertPullRequestURLtoNumber(runevent.URL)
	if err != nil {
		return nil, err
	}

	comments, _, err := v.Client.Issues.ListComments(ctx, runevent.Organization, runevent.Repository,
		prNumber, &github.IssueListCommentsOptions{})
	if err != nil {
		return nil, err
	}
	for _, v := range comments {
		if acl.MatchRegexp(reg, v.GetBody()) {
			ret = append(ret, v)
		}
	}
	return ret, nil
}
