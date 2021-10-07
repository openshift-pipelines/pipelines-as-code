package bitbucketcloud

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs/bitbucketcloud/types"
)

const bitbucketCloudIPrangesList = "https://ip-ranges.atlassian.com/"

// lastForwarderForIP get last ip from the X-Forwarded-For chain
// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/X-Forwarded-For
func lastForwarderForIP(xff string) string {
	splitted := strings.Split(xff, ",")
	return splitted[len(splitted)-1]
}

// checkFromPublicCloudIPS Grab public IP from public cloud and make sure we match it
func (v *VCS) checkFromPublicCloudIPS(ctx context.Context, run *params.Run) (bool, error) {
	enval, ok := os.LookupEnv("PAC_BITBUCKET_CLOUD_CHECK_SOURCE_IP")
	if !ok || strings.ToLower(enval) != "true" {
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

func (v *VCS) ParsePayload(ctx context.Context, run *params.Run, payload string) (*info.Event, error) {
	processedevent := run.Info.Event
	event, err := parsePayloadType(run.Info.Event.EventType, []byte(payload))
	if err != nil {
		return &info.Event{}, err
	}

	err = json.Unmarshal([]byte(payload), &event)
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

	processedevent.Event = event

	switch e := event.(type) {
	case *types.PullRequestEvent:
		processedevent.Owner = e.Repository.Workspace.Slug
		processedevent.Repository = e.Repository.Name
		processedevent.SHA = e.PullRequest.Source.Commit.Hash
		processedevent.URL = e.Repository.Links.HTML.HRef
		processedevent.BaseBranch = e.PullRequest.Destination.Branch.Name
		processedevent.HeadBranch = e.PullRequest.Source.Branch.Name
		processedevent.AccountID = e.PullRequest.Author.AccountID
		processedevent.Sender = e.PullRequest.Author.Nickname
	case *types.PushRequestEvent:
		processedevent.Owner = e.Repository.Workspace.Slug
		processedevent.Repository = e.Repository.Name
		processedevent.SHA = e.Push.Changes[0].New.Target.Hash
		processedevent.URL = e.Repository.Links.HTML.HRef
		processedevent.BaseBranch = e.Push.Changes[0].New.Name
		processedevent.HeadBranch = e.Push.Changes[0].Old.Name
		processedevent.AccountID = e.Actor.AccountID
		processedevent.Sender = e.Actor.Nickname
	default:
		return nil, fmt.Errorf("event %s is not recognized", run.Info.Event.EventType)
	}
	return processedevent, nil
}
