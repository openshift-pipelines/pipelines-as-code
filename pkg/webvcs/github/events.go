package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/google/go-github/v35/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"go.uber.org/zap"
)

// payloadFix since we are getting a bunch of \r\n or \n and others from triggers/github, so let just
// workaround it. Originally from https://stackoverflow.com/a/52600147
func (v VCS) payloadFix(payload string) []byte {
	replacement := " "
	replacer := strings.NewReplacer(
		"\r\n", replacement,
		"\r", replacement,
		"\n", replacement,
		"\v", replacement,
		"\f", replacement,
		"\u0085", replacement,
		"\u2028", replacement,
		"\u2029", replacement,
	)
	return []byte(replacer.Replace(payload))
}

// ParsePayload parse payload event
// TODO: this piece of code is just plain silly
func (v VCS) ParsePayload(ctx context.Context, log *zap.SugaredLogger, runevent *info.Event, payload string) (*info.Event, error) {
	var processedevent *info.Event
	payloadTreated := v.payloadFix(payload)
	event, err := github.ParseWebHook(runevent.EventType, payloadTreated)
	if err != nil {
		return &info.Event{}, err
	}

	err = json.Unmarshal(payloadTreated, &event)
	if err != nil {
		return &info.Event{}, err
	}

	switch event := event.(type) {
	case *github.CheckRunEvent:
		if runevent.TriggerTarget == "issue-recheck" {
			processedevent, err = v.handleReRequestEvent(ctx, log, event)
			if err != nil {
				return &info.Event{}, err
			}
		}
	case *github.IssueCommentEvent:
		processedevent, err = v.handleIssueCommentEvent(ctx, log, event)
		if err != nil {
			return &info.Event{}, err
		}
	case *github.PushEvent:
		processedevent = &info.Event{
			Owner:         event.GetRepo().GetOwner().GetLogin(),
			Repository:    event.GetRepo().GetName(),
			DefaultBranch: event.GetRepo().GetDefaultBranch(),
			URL:           event.GetRepo().GetHTMLURL(),
			SHA:           event.GetHeadCommit().GetID(),
			SHAURL:        event.GetHeadCommit().GetURL(),
			SHATitle:      event.GetHeadCommit().GetMessage(),
			Sender:        event.GetSender().GetLogin(),
			BaseBranch:    event.GetRef(),
			EventType:     runevent.TriggerTarget,
		}

		processedevent.HeadBranch = processedevent.BaseBranch // in push events Head Branch is the same as Basebranch
	case *github.PullRequestEvent:
		processedevent = &info.Event{
			Owner:         event.GetRepo().Owner.GetLogin(),
			Repository:    event.GetRepo().GetName(),
			DefaultBranch: event.GetRepo().GetDefaultBranch(),
			SHA:           event.GetPullRequest().Head.GetSHA(),
			URL:           event.GetRepo().GetHTMLURL(),
			BaseBranch:    event.GetPullRequest().Base.GetRef(),
			HeadBranch:    event.GetPullRequest().Head.GetRef(),
			Sender:        event.GetPullRequest().GetUser().GetLogin(),
			EventType:     runevent.EventType,
		}

	default:
		return &info.Event{}, errors.New("this event is not supported")
	}

	err = v.populateCommitInfo(ctx, processedevent)
	if err != nil {
		return &info.Event{}, err
	}

	processedevent.Event = event
	processedevent.TriggerTarget = runevent.TriggerTarget

	return processedevent, nil
}

func (v VCS) handleReRequestEvent(ctx context.Context, log *zap.SugaredLogger, event *github.CheckRunEvent) (*info.Event, error) {
	runevent := &info.Event{
		Owner:         event.GetRepo().GetOwner().GetLogin(),
		Repository:    event.GetRepo().GetName(),
		URL:           event.GetRepo().GetHTMLURL(),
		DefaultBranch: event.GetRepo().GetDefaultBranch(),
		SHA:           event.GetCheckRun().GetCheckSuite().GetHeadSHA(),
		HeadBranch:    event.GetCheckRun().GetCheckSuite().GetHeadBranch(),
	}

	// If we don't have a pull_request in this it probably mean a push
	if len(event.GetCheckRun().GetCheckSuite().PullRequests) == 0 {
		runevent.BaseBranch = runevent.HeadBranch
		runevent.EventType = "push"
		// we allow the rerequest user here, not the push user, i guess it's
		// fine because you can't do a rereq without being a github owner?
		runevent.Sender = event.GetSender().GetLogin()
		return runevent, nil
	}
	prNumber := event.GetCheckRun().GetCheckSuite().PullRequests[0].GetNumber()
	log.Infof("Recheck of PR %s/%s#%d has been requested", runevent.Owner, runevent.Repository, prNumber)
	return v.getPullRequest(ctx, runevent, prNumber)
}

func convertPullRequestURLtoNumber(pullRequest string) (int, error) {
	prNumber, err := strconv.Atoi(path.Base(pullRequest))
	if err != nil {
		return -1, err
	}
	return prNumber, nil
}

func (v VCS) handleIssueCommentEvent(ctx context.Context, log *zap.SugaredLogger, event *github.IssueCommentEvent) (*info.Event, error) {
	runevent := &info.Event{
		Owner:      event.GetRepo().GetOwner().GetLogin(),
		Repository: event.GetRepo().GetName(),
	}

	if !event.GetIssue().IsPullRequest() {
		return &info.Event{}, fmt.Errorf("issue comment is not coming from a pull_request")
	}

	// We are getting the full URL so we have to get the last part to get the PR number,
	// we don't have to care about URL query string/hash and other stuff because
	// that comes up from the API.
	prNumber, err := convertPullRequestURLtoNumber(event.GetIssue().GetPullRequestLinks().GetHTMLURL())
	if err != nil {
		return &info.Event{}, err
	}

	log.Infof("PR recheck from issue commment on %s/%s#%d has been requested", runevent.Owner, runevent.Repository, prNumber)
	return v.getPullRequest(ctx, runevent, prNumber)
}
