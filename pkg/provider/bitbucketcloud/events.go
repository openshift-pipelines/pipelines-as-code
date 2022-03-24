package bitbucketcloud

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/bitbucketcloud/types"
)

const bitbucketCloudIPrangesList = "https://ip-ranges.atlassian.com/"

// lastForwarderForIP get last ip from the X-Forwarded-For chain
// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/X-Forwarded-For
func lastForwarderForIP(xff string) string {
	splitted := strings.Split(xff, ",")
	return splitted[len(splitted)-1]
}

// checkFromPublicCloudIPS Grab public IP from public cloud and make sure we match it
func (v *Provider) checkFromPublicCloudIPS(ctx context.Context, run *params.Run) (bool, error) {
	enval, ok := os.LookupEnv("PAC_BITBUCKET_CLOUD_CHECK_SOURCE_IP")
	if !ok || !params.StringToBool(enval) {
		return true, nil
	}

	sourceIP, ok := os.LookupEnv("PAC_SOURCE_IP")
	if !ok {
		return false, fmt.Errorf("we need to check the source_ip but no source_ip has been passed")
	}
	sourceIP = lastForwarderForIP(sourceIP)

	netsourceIP := net.ParseIP(sourceIP)
	data, err := run.Clients.GetURL(ctx, bitbucketCloudIPrangesList)
	if err != nil {
		return false, err
	}

	ipranges := &types.IPRanges{}
	err = json.Unmarshal(data, &ipranges)
	if err != nil {
		return false, err
	}

	extraIPEnv, _ := os.LookupEnv("PAC_BITBUCKET_CLOUD_ADDITIONAL_SOURCE_IP")
	if extraIPEnv != "" {
		for _, value := range strings.Split(extraIPEnv, ",") {
			if !strings.Contains(value, "/") {
				value = fmt.Sprintf("%s/32", value)
			}
			ipranges.Items = append(ipranges.Items, types.IPRangesItem{
				CIDR: strings.TrimSpace(value),
			})
		}
	}
	for _, value := range ipranges.Items {
		_, cidr, err := net.ParseCIDR(value.CIDR)
		if err != nil {
			return false, err
		}
		if cidr.Contains(netsourceIP) {
			return true, nil
		}
	}
	return false,
		fmt.Errorf("payload from %s is not coming from the public bitbucket cloud ips as defined here: %s",
			sourceIP, bitbucketCloudIPrangesList)
}

func parsePayloadType(messageType string, rawPayload []byte) (interface{}, error) {
	var payload interface{}

	switch messageType {
	case "pull_request":
		payload = &types.PullRequestEvent{}
	case "push":
		payload = &types.PushRequestEvent{}
	}
	err := json.Unmarshal(rawPayload, payload)
	return payload, err
}

func (v *Provider) ParsePayload(ctx context.Context, run *params.Run, request *http.Request, payload string) (*info.Event, error) {
	// TODO: parse request to figure out which event
	event := &info.Event{}
	processedEvent := event
	eventInt, err := parsePayloadType(event.EventType, []byte(payload))
	if err != nil {
		return &info.Event{}, err
	}

	err = json.Unmarshal([]byte(payload), &eventInt)
	if err != nil {
		return &info.Event{}, err
	}

	allowed, err := v.checkFromPublicCloudIPS(ctx, run)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, fmt.Errorf("payload is not coming from the public bitbucket cloud ips as defined here: %s",
			bitbucketCloudIPrangesList)
	}

	processedEvent.Event = eventInt

	switch e := eventInt.(type) {
	case *types.PullRequestEvent:
		processedEvent.Organization = e.Repository.Workspace.Slug
		processedEvent.Repository = e.Repository.Name
		processedEvent.SHA = e.PullRequest.Source.Commit.Hash
		processedEvent.URL = e.Repository.Links.HTML.HRef
		processedEvent.BaseBranch = e.PullRequest.Destination.Branch.Name
		processedEvent.HeadBranch = e.PullRequest.Source.Branch.Name
		processedEvent.AccountID = e.PullRequest.Author.AccountID
		processedEvent.Sender = e.PullRequest.Author.Nickname
	case *types.PushRequestEvent:
		processedEvent.Organization = e.Repository.Workspace.Slug
		processedEvent.Repository = e.Repository.Name
		processedEvent.SHA = e.Push.Changes[0].New.Target.Hash
		processedEvent.URL = e.Repository.Links.HTML.HRef
		processedEvent.BaseBranch = e.Push.Changes[0].New.Name
		processedEvent.HeadBranch = e.Push.Changes[0].Old.Name
		processedEvent.AccountID = e.Actor.AccountID
		processedEvent.Sender = e.Actor.Nickname
	default:
		return nil, fmt.Errorf("event %s is not recognized", event.EventType)
	}
	return processedEvent, nil
}
