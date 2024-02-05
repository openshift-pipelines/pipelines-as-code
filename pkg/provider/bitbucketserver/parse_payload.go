package bitbucketserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/bitbucketserver/types"
)

// sanitizeEventURL returns the URL to the event without the /browse.
func sanitizeEventURL(eventURL string) string {
	if strings.HasSuffix(eventURL, "/browse") {
		return eventURL[:len(eventURL)-len("/browse")]
	}
	return eventURL
}

// sanitizeOwner remove ~ from OWNER in case of personal repos.
func sanitizeOwner(owner string) string {
	return strings.ReplaceAll(owner, "~", "")
}

// ParsePayload parses the payload from the event.
func (v *Provider) ParsePayload(_ context.Context, _ *params.Run, request *http.Request,
	payload string,
) (*info.Event, error) {
	processedEvent := info.NewEvent()

	eventType := request.Header.Get("X-Event-Key")
	eventPayload, err := parsePayloadType(eventType)
	if err != nil {
		return info.NewEvent(), err
	}

	if err := json.Unmarshal([]byte(payload), &eventPayload); err != nil {
		return info.NewEvent(), err
	}

	processedEvent.Event = eventPayload

	switch e := eventPayload.(type) {
	case *types.PullRequestEvent:
		if provider.Valid(eventType, []string{"pr:from_ref_updated", "pr:opened"}) {
			processedEvent.TriggerTarget = triggertype.PullRequest
			processedEvent.EventType = triggertype.PullRequest.String()
		} else if provider.Valid(eventType, []string{"pr:comment:added", "pr:comment:edited"}) {
			switch {
			case provider.IsTestRetestComment(e.Comment.Text):
				processedEvent.TriggerTarget = triggertype.PullRequest
				if strings.Contains(e.Comment.Text, "/test") {
					processedEvent.EventType = "test-comment"
				} else {
					processedEvent.EventType = "retest-comment"
				}
				processedEvent.TargetTestPipelineRun = provider.GetPipelineRunFromTestComment(e.Comment.Text)
			case provider.IsOkToTestComment(e.Comment.Text):
				processedEvent.TriggerTarget = triggertype.PullRequest
				processedEvent.EventType = "ok-to-test-comment"
			case provider.IsCancelComment(e.Comment.Text):
				processedEvent.TriggerTarget = triggertype.PullRequest
				processedEvent.EventType = "cancel-comment"
				processedEvent.CancelPipelineRuns = true
				processedEvent.TargetCancelPipelineRun = provider.GetPipelineRunFromCancelComment(e.Comment.Text)
			}
		}
		// TODO: It's Really not an OWNER but a PROJECT
		processedEvent.Organization = e.PulRequest.ToRef.Repository.Project.Key
		processedEvent.Repository = e.PulRequest.ToRef.Repository.Name
		processedEvent.SHA = e.PulRequest.FromRef.LatestCommit
		processedEvent.PullRequestNumber = e.PulRequest.ID
		processedEvent.URL = e.PulRequest.ToRef.Repository.Links.Self[0].Href
		processedEvent.BaseBranch = e.PulRequest.ToRef.DisplayID
		processedEvent.HeadBranch = e.PulRequest.FromRef.DisplayID
		processedEvent.BaseURL = e.PulRequest.ToRef.Repository.Links.Self[0].Href
		processedEvent.HeadURL = e.PulRequest.FromRef.Repository.Links.Self[0].Href
		processedEvent.AccountID = fmt.Sprintf("%d", e.Actor.ID)
		processedEvent.Sender = e.Actor.Name
		for _, value := range e.PulRequest.FromRef.Repository.Links.Clone {
			if value.Name == "http" {
				processedEvent.CloneURL = value.Href
			}
		}
		v.pullRequestNumber = e.PulRequest.ID
	case *types.PushRequestEvent:
		processedEvent.Event = "push"
		processedEvent.TriggerTarget = "push"
		processedEvent.Organization = e.Repository.Project.Key
		processedEvent.Repository = e.Repository.Slug
		processedEvent.SHA = e.Changes[0].ToHash
		processedEvent.URL = e.Repository.Links.Self[0].Href
		processedEvent.BaseBranch = e.Changes[0].RefID
		processedEvent.HeadBranch = e.Changes[0].RefID
		processedEvent.BaseURL = e.Repository.Links.Self[0].Href
		processedEvent.HeadURL = e.Repository.Links.Self[0].Href
		processedEvent.AccountID = fmt.Sprintf("%d", e.Actor.ID)
		processedEvent.Sender = e.Actor.Name
		// Should we care about clone via SSH or just only do HTTP clones?
		for _, value := range e.Repository.Links.Clone {
			if value.Name == "http" {
				processedEvent.CloneURL = value.Href
			}
		}
	default:
		return nil, fmt.Errorf("event %s is not supported", eventType)
	}

	v.projectKey = processedEvent.Organization
	processedEvent.Organization = sanitizeOwner(processedEvent.Organization)
	processedEvent.URL = sanitizeEventURL(processedEvent.URL)

	// TODO: is this the right way? I guess i have no way to know what is the
	// baseURL of a server unless there is something in the API?
	// remove everything after /project in the URL to get the basePath
	pURL, err := url.Parse(processedEvent.URL)
	if err != nil {
		return nil, err
	}

	v.baseURL = fmt.Sprintf("%s://%s", pURL.Scheme, pURL.Host)
	return processedEvent, nil
}

func parsePayloadType(event string) (interface{}, error) {
	// bitbucket server event type has `pr:` prefix for pull request
	// but in case of push event it is `repo:` prefix for both bitbucket server
	// and cloud, so we check the event name directly
	var localEvent string
	if strings.HasPrefix(event, "pr:") {
		if !provider.Valid(event, []string{
			"pr:from_ref_updated", "pr:opened", "pr:comment:added", "pr:comment:edited",
		}) {
			return nil, fmt.Errorf("event \"%s\" is not supported", event)
		}
		localEvent = triggertype.PullRequest.String()
	} else if event == "repo:refs_changed" {
		localEvent = "push"
	}

	var intfType interface{}
	switch localEvent {
	case triggertype.PullRequest.String():
		intfType = &types.PullRequestEvent{}
	case "push":
		intfType = &types.PushRequestEvent{}
	default:
		intfType = nil
	}
	return intfType, nil
}
