package github

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/google/go-github/v35/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"sigs.k8s.io/yaml"
)

var okToTestCommentRegexp = `(^|\n)/ok-to-test(\r\n|$)`

func (v VCS) IsAllowed(ctx context.Context, event *info.Event) (bool, error) {
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
func (v VCS) aclAllowedOkToTestFromAnOwner(ctx context.Context, event *info.Event) (bool, error) {
	revent := event
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

	comments, err := v.GetStringPullRequestComment(ctx, revent, okToTestCommentRegexp)
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

// ownersConfig prow owner, only supporting approvers or reviewers in yaml
type ownersConfig struct {
	Approvers []string `json:"approvers,omitempty"`
	Reviewers []string `json:"reviewers,omitempty"`
}

// aclCheck check if we are allowed to run the pipeline on that PR
func (v VCS) aclCheckAll(ctx context.Context, rev *info.Event) (bool, error) {
	if rev.Owner == rev.Sender {
		return true, nil
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

	// If we have a prow OWNERS file in the defaultBranch (ie: master) then
	// parse it in approvers and reviewers field and check if sender is in there.
	ownerFile, err := v.getFileFromDefaultBranch(ctx, "OWNERS", rev)

	// Don't error out if the OWNERS file cannot be found
	if err != nil && !strings.Contains(err.Error(), "cannot find") {
		return false, err
	} else if ownerFile != "" {
		var ownerConfig ownersConfig
		err := yaml.Unmarshal([]byte(ownerFile), &ownerConfig)
		if err != nil {
			return false, err
		}
		for _, owner := range append(ownerConfig.Approvers, ownerConfig.Reviewers...) {
			if owner == rev.Sender {
				return true, nil
			}
		}
	}

	return false, nil
}

// checkSenderOrgMembership Get sender user's organization. We can
// only get the one that the user sets as public 🤷
func (v VCS) checkSenderOrgMembership(ctx context.Context, runevent *info.Event) (bool, error) {
	users, resp, err := v.Client.Organizations.ListMembers(ctx, runevent.Owner,
		&github.ListMembersOptions{
			PublicOnly: true, // We can't list private member in a org
		})
	// If we are 404 it means we are checking a repo owner and not a org so let's bail out with grace
	if resp != nil && resp.Response.StatusCode == http.StatusNotFound {
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

// getFileFromDefaultBranch will get a file directly from the Default BaseBranch as
// configured in runinfo which is directly set in webhook by Github
func (v VCS) getFileFromDefaultBranch(ctx context.Context, path string, runevent *info.Event) (string, error) {
	tektonyaml, err := v.GetFileInsideRepo(ctx, runevent, path, runevent.DefaultBranch)
	if err != nil {
		return "", fmt.Errorf("cannot find %s inside the %s branch: %w", path, runevent.DefaultBranch, err)
	}
	return tektonyaml, err
}

// GetStringPullRequestComment return the comment if we find a regexp in one of
// the comments text of a pull request
func (v VCS) GetStringPullRequestComment(ctx context.Context, runevent *info.Event, reg string) ([]*github.IssueComment, error) {
	var ret []*github.IssueComment
	prNumber, err := convertPullRequestURLtoNumber(runevent.URL)
	if err != nil {
		return nil, err
	}
	comments, _, err := v.Client.Issues.ListComments(ctx, runevent.Owner, runevent.Repository,
		prNumber, &github.IssueListCommentsOptions{})
	if err != nil {
		return nil, err
	}

	re := regexp.MustCompile(reg)
	for _, v := range comments {
		if string(re.Find([]byte(v.GetBody()))) != "" {
			ret = append(ret, v)
		}
	}
	return ret, nil
}
