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
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/bitbucketserver/types"
)

// sanitizeEventURL returns the URL to the event without the /browse
func sanitizeEventURL(eventURL string) string {
	if strings.HasSuffix(eventURL, "/browse") {
		return eventURL[:len(eventURL)-len("/browse")]
	}
	return eventURL
}

// sanitizeOwner remove ~ from OWNER in case of personal repos
func sanitizeOwner(owner string) string {
	return strings.ReplaceAll(owner, "~", "")
}

// ParsePayload parses the payload from the event
func (v *Provider) ParsePayload(ctx context.Context, run *params.Run, request *http.Request, payload string) (*info.Event, error) {
	// TODO: parse request to figure out which event
	event := &info.Event{}
	processedEvent := event

	eventPayload := parsePayloadType(event.EventType)
	if err := json.Unmarshal([]byte(payload), &eventPayload); err != nil {
		return &info.Event{}, err
	}

	processedEvent.Event = eventPayload

	switch e := eventPayload.(type) {
	case *types.PullRequestEvent:
		// TODO: It's Really not an OWNER but a PROJECT
		processedEvent.Organization = e.PulRequest.ToRef.Repository.Project.Key
		processedEvent.Repository = e.PulRequest.ToRef.Repository.Name
		processedEvent.SHA = e.PulRequest.FromRef.LatestCommit
		processedEvent.URL = e.PulRequest.ToRef.Repository.Links.Self[0].Href
		processedEvent.BaseBranch = e.PulRequest.ToRef.DisplayID
		processedEvent.HeadBranch = e.PulRequest.FromRef.DisplayID
		processedEvent.AccountID = fmt.Sprintf("%d", e.Actor.ID)
		processedEvent.Sender = e.Actor.Name
		for _, value := range e.PulRequest.FromRef.Repository.Links.Clone {
			if value.Name == "http" {
				processedEvent.CloneURL = value.Href
			}
		}
		v.pullRequestNumber = e.PulRequest.ID
	case *types.PushRequestEvent:
		processedEvent.Organization = e.Repository.Project.Key
		processedEvent.Repository = e.Repository.Slug
		processedEvent.SHA = e.Changes[0].ToHash
		processedEvent.URL = e.Repository.Links.Self[0].Href
		processedEvent.BaseBranch = e.Changes[0].RefID
		processedEvent.HeadBranch = e.Changes[0].RefID
		processedEvent.AccountID = fmt.Sprintf("%d", e.Actor.ID)
		processedEvent.Sender = e.Actor.Name
		// Should we care about clone via SSH or just only do HTTP clones?
		for _, value := range e.Repository.Links.Clone {
			if value.Name == "http" {
				processedEvent.CloneURL = value.Href
			}
		}
	default:
		return nil, fmt.Errorf("event %s is not supported", event.EventType)
	}

	v.projectKey = processedEvent.Organization
	processedEvent.Organization = sanitizeOwner(processedEvent.Organization)
	processedEvent.URL = sanitizeEventURL(processedEvent.URL)

	// TODO: is this the right way? I guess i have no way to know what is the
	// baseURL of a server unless there is something in the API?
	// remove everything after /project in the URL to get the basePath
	url, err := url.Parse(processedEvent.URL)
	if err != nil {
		return nil, err
	}

	v.baseURL = fmt.Sprintf("%s://%s", url.Scheme, url.Host)
	return processedEvent, nil
}

func parsePayloadType(messageType string) interface{} {
	var intfType interface{}
	switch messageType {
	case "pull_request":
		intfType = &types.PullRequestEvent{}
	case "push":
		intfType = &types.PushRequestEvent{}
	}
	return intfType
}
