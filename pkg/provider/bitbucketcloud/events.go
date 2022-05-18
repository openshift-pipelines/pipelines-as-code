package bitbucketcloud

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/bitbucketcloud/types"
	"go.uber.org/zap"
)

const bitbucketCloudIPrangesList = "https://ip-ranges.atlassian.com/"

// lastForwarderForIP get last ip from the X-Forwarded-For chain
// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/X-Forwarded-For
func lastForwarderForIP(xff string) string {
	splitted := strings.Split(xff, ",")
	return splitted[len(splitted)-1]
}

// checkFromPublicCloudIPS Grab public IP from public cloud and make sure we match it
func (v *Provider) checkFromPublicCloudIPS(ctx context.Context, run *params.Run, sourceIP string) (bool, error) {
	if !run.Info.Pac.BitbucketCloudCheckSourceIP {
		return true, nil
	}

	if sourceIP == "" {
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

	extraIPEnv := run.Info.Pac.BitbucketCloudAdditionalSourceIP
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

func parsePayloadType(event string, rawPayload string) (interface{}, error) {
	var payload interface{}

	var localEvent string
	if strings.HasPrefix(event, "pullrequest:") {
		if !provider.Valid(event, []string{
			"pullrequest:created", "pullrequest:updated", "pullrequest:comment_created",
		}) {
			return nil, fmt.Errorf("event %s is not supported", event)
		}
		localEvent = "pull_request"
	} else if event == "repo:push" {
		localEvent = "push"
	}

	switch localEvent {
	case "pull_request":
		payload = &types.PullRequestEvent{}
	case "push":
		payload = &types.PushRequestEvent{}
	default:
		return nil, nil
	}
	err := json.Unmarshal([]byte(rawPayload), payload)
	return payload, err
}

func (v *Provider) ParsePayload(ctx context.Context, run *params.Run, request *http.Request, payload string) (*info.Event, error) {
	processedEvent := info.NewEvent()

	event := request.Header.Get("X-Event-Key")
	eventInt, err := parsePayloadType(event, payload)
	if err != nil || eventInt == nil {
		return info.NewEvent(), err
	}

	err = json.Unmarshal([]byte(payload), &eventInt)
	if err != nil {
		return info.NewEvent(), err
	}

	sourceIP := request.Header.Get("X-Forwarded-For")
	allowed, err := v.checkFromPublicCloudIPS(ctx, run, sourceIP)
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
		if provider.Valid(event, []string{"pullrequest:created", "pullrequest:updated"}) {
			processedEvent.TriggerTarget = "pull_request"
			processedEvent.EventType = "pull_request"
		} else if provider.Valid(event, []string{"pullrequest:comment_created"}) {
			switch {
			case provider.IsTestRetestComment(e.Comment.Content.Raw):
				processedEvent.TriggerTarget = "pull_request"
				if strings.Contains(e.Comment.Content.Raw, "/test") {
					processedEvent.EventType = "test-comment"
				} else {
					processedEvent.EventType = "retest-comment"
				}
				processedEvent.TargetTestPipelineRun = provider.GetPipelineRunFromComment(e.Comment.Content.Raw)
			case provider.IsOkToTestComment(e.Comment.Content.Raw):
				processedEvent.TriggerTarget = "pull_request"
				processedEvent.EventType = "ok-to-test-comment"
			}
		}
		processedEvent.Organization = e.Repository.Workspace.Slug
		processedEvent.Repository = e.Repository.Name
		processedEvent.SHA = e.PullRequest.Source.Commit.Hash
		processedEvent.URL = e.Repository.Links.HTML.HRef
		processedEvent.BaseBranch = e.PullRequest.Destination.Branch.Name
		processedEvent.HeadBranch = e.PullRequest.Source.Branch.Name
		processedEvent.AccountID = e.PullRequest.Author.AccountID
		processedEvent.Sender = e.PullRequest.Author.Nickname
		processedEvent.PullRequestNumber = e.PullRequest.ID
	case *types.PushRequestEvent:
		processedEvent.Event = "push"
		processedEvent.TriggerTarget = "push"
		processedEvent.Organization = e.Repository.Workspace.Slug
		processedEvent.Repository = e.Repository.Name
		processedEvent.SHA = e.Push.Changes[0].New.Target.Hash
		processedEvent.URL = e.Repository.Links.HTML.HRef
		processedEvent.BaseBranch = e.Push.Changes[0].New.Name
		processedEvent.HeadBranch = e.Push.Changes[0].Old.Name
		processedEvent.AccountID = e.Actor.AccountID
		processedEvent.Sender = e.Actor.Nickname
	default:
		return nil, fmt.Errorf("event %s is not recognized", event)
	}
	return processedEvent, nil
}

func (v *Provider) Detect(req *http.Request, payload string, logger *zap.SugaredLogger) (bool, bool, *zap.SugaredLogger, string, error) {
	isBitCloud := false
	reqHeader := req.Header
	event := reqHeader.Get("X-Event-Key")
	if event == "" {
		return false, false, logger, "", nil
	}

	eventInt, err := parsePayloadType(event, payload)
	if err != nil || eventInt == nil {
		return false, false, logger, "", err
	}

	// it is a Bitbucket cloud event
	isBitCloud = true

	setLoggerAndProceed := func(processEvent bool, reason string, err error) (bool, bool, *zap.SugaredLogger,
		string, error,
	) {
		logger = logger.With("provider", "bitbucket-cloud", "event-id", reqHeader.Get("X-Request-Id"))
		return isBitCloud, processEvent, logger, reason, err
	}

	_ = json.Unmarshal([]byte(payload), &eventInt)

	switch e := eventInt.(type) {
	case *types.PullRequestEvent:
		if provider.Valid(event, []string{"pullrequest:created", "pullrequest:updated"}) {
			return setLoggerAndProceed(true, "", nil)
		}
		if provider.Valid(event, []string{"pullrequest:comment_created"}) {
			if provider.IsTestRetestComment(e.Comment.Content.Raw) {
				return setLoggerAndProceed(true, "", nil)
			}
			if provider.IsOkToTestComment(e.Comment.Content.Raw) {
				return setLoggerAndProceed(true, "", nil)
			}
		}
		return setLoggerAndProceed(false, fmt.Sprintf("not a valid gitops comment: \"%s\"", event), nil)

	case *types.PushRequestEvent:
		if provider.Valid(event, []string{"repo:push"}) {
			if e.Push.Changes != nil {
				return setLoggerAndProceed(true, "", nil)
			}
		}
		return setLoggerAndProceed(false, fmt.Sprintf("invalid push event: \"%s\"", event), nil)

	default:
		return setLoggerAndProceed(false, "", fmt.Errorf("bitbucket-cloud: event \"%s\" is not supported", event))
	}
}
