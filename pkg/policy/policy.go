package policy

import (
	"context"
	"fmt"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/events"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"go.uber.org/zap"
)

type Result int

const (
	ResultNotSet     Result = 0
	ResultAllowed    Result = 1
	ResultDisallowed Result = 2
)

type Policy struct {
	Repository   *v1alpha1.Repository
	Event        *info.Event
	VCX          provider.Interface
	Logger       *zap.SugaredLogger
	EventEmitter *events.EventEmitter
}

// checkAllowed checks if the policy is set and allows the event to be processed.
func (p *Policy) checkAllowed(ctx context.Context, tType triggertype.Trigger) (Result, string) {
	if p.Repository == nil {
		return ResultNotSet, ""
	}
	settings := p.Repository.Spec.Settings
	if settings == nil || settings.Policy == nil {
		return ResultNotSet, ""
	}

	var sType []string
	switch tType {
	// NOTE: This make /retest /ok-to-test /test bound to the same policy, which is fine from a security standpoint but maybe we want to refine this in the future.
	case triggertype.OkToTest, triggertype.Retest:
		sType = settings.Policy.OkToTest
	// apply the same policy for PullRequest and comment
	// we don't support comments on PRs yet but if we do on the future we will need our own policy
	case triggertype.PullRequest, triggertype.Comment, triggertype.PullRequestLabeled, triggertype.PullRequestClosed:
		sType = settings.Policy.PullRequest
	// NOTE: not supported yet, will imp if it gets requested and reasonable to implement
	case triggertype.Push, triggertype.Cancel, triggertype.CheckSuiteRerequested, triggertype.CheckRunRerequested, triggertype.Incoming:
		return ResultNotSet, ""
	default:
		return ResultNotSet, ""
	}

	// if policy is set but empty then it mean disallow everything
	if len(sType) == 0 {
		return ResultDisallowed, "no policy set"
	}

	// remove empty values from sType
	temp := []string{}
	for _, val := range sType {
		if val != "" {
			temp = append(temp, val)
		}
	}
	sType = temp

	// if policy is set but with empty values then bail out.
	if len(sType) == 0 {
		return ResultDisallowed, "policy set and empty with no groups"
	}

	allowed, reason := p.VCX.CheckPolicyAllowing(ctx, p.Event, sType)
	if allowed {
		return ResultAllowed, ""
	}
	return ResultDisallowed, fmt.Sprintf("policy check: %s, %s", string(tType), reason)
}

func (p *Policy) IsAllowed(ctx context.Context, tType triggertype.Trigger) (Result, string) {
	var reason string
	policyRes, reason := p.checkAllowed(ctx, tType)
	switch policyRes {
	case ResultAllowed:
		reason = fmt.Sprintf("policy check: policy is set for sender %s has been allowed to run CI via policy", p.Event.Sender)
		p.EventEmitter.EmitMessage(p.Repository, zap.InfoLevel, "PolicySetAllowed", reason)
		return ResultAllowed, ""
	case ResultDisallowed:
		allowed, err := p.VCX.IsAllowedOwnersFile(ctx, p.Event)
		if err != nil {
			return ResultDisallowed, err.Error()
		}
		if allowed {
			reason = fmt.Sprintf("policy check: policy is set, sender %s not in the allowed policy but allowed via OWNERS file", p.Event.Sender)
			p.EventEmitter.EmitMessage(p.Repository, zap.InfoLevel, "PolicySetAllowed", reason)
			return ResultAllowed, ""
		}
		if reason == "" {
			reason = fmt.Sprintf("policy check: policy is set but sender %s is not in the allowed groups", p.Event.Sender)
		}
		p.EventEmitter.EmitMessage(p.Repository, zap.InfoLevel, "PolicySetDisallowed", reason)
		return ResultDisallowed, ""
	case ResultNotSet: // this is to make golangci-lint happy
	}
	return ResultNotSet, reason
}
