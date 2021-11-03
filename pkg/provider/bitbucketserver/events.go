package bitbucketserver

import (
	"context"
	"encoding/json"
	"fmt"
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
func (v *Provider) ParsePayload(ctx context.Context, run *params.Run, payload string) (*info.Event, error) {
	processedevent := run.Info.Event

	eventPayload := parsePayloadType(run.Info.Event.EventType)
	if err := json.Unmarshal([]byte(payload), &eventPayload); err != nil {
		return &info.Event{}, err
	}

	processedevent.Event = eventPayload

	switch e := eventPayload.(type) {
	case *types.PullRequestEvent:
		// TODO: It's Really not an OWNER but a PROJECT
		processedevent.Organization = e.PulRequest.ToRef.Repository.Project.Key
		processedevent.Repository = e.PulRequest.ToRef.Repository.Name
		processedevent.SHA = e.PulRequest.FromRef.LatestCommit
		processedevent.URL = e.PulRequest.ToRef.Repository.Links.Self[0].Href
		processedevent.BaseBranch = e.PulRequest.ToRef.DisplayID
		processedevent.HeadBranch = e.PulRequest.FromRef.DisplayID
		processedevent.AccountID = fmt.Sprintf("%d", e.Actor.ID)
		processedevent.Sender = e.Actor.Name
		for _, value := range e.PulRequest.FromRef.Repository.Links.Clone {
			if value.Name == "http" {
				processedevent.CloneURL = value.Href
			}
		}
		v.pullRequestNumber = e.PulRequest.ID
	case *types.PushRequestEvent:
		processedevent.Organization = e.Repository.Project.Key
		processedevent.Repository = e.Repository.Slug
		processedevent.SHA = e.Changes[0].ToHash
		processedevent.URL = e.Repository.Links.Self[0].Href
		processedevent.BaseBranch = e.Changes[0].RefID
		processedevent.HeadBranch = e.Changes[0].RefID
		processedevent.AccountID = fmt.Sprintf("%d", e.Actor.ID)
		processedevent.Sender = e.Actor.Name
		// Should we care about clone via SSH or just only do HTTP clones?
		for _, value := range e.Repository.Links.Clone {
			if value.Name == "http" {
				processedevent.CloneURL = value.Href
			}
		}
	default:
		return nil, fmt.Errorf("event %s is not supported", run.Info.Event.EventType)
	}

	v.projectKey = processedevent.Organization
	processedevent.Organization = sanitizeOwner(processedevent.Organization)
	processedevent.URL = sanitizeEventURL(processedevent.URL)

	// TODO: is this the right way? I guess i have no way to know what is the
	// baseURL of a server unless there is something in the API?
	// remove everything after /project in the URL to get the basePath
	url, err := url.Parse(processedevent.URL)
	if err != nil {
		return nil, err
	}

	v.baseURL = fmt.Sprintf("%s://%s", url.Scheme, url.Host)
	return processedevent, nil
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
