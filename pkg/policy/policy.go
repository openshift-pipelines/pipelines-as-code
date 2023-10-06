package policy

import (
	"context"
	"fmt"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/events"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
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

func (p *Policy) checkAllowed(ctx context.Context, tType info.TriggerType) (Result, error) {
	if p.Repository == nil {
		return ResultNotSet, nil
	}
	settings := p.Repository.Spec.Settings
	if settings == nil || settings.Policy == nil {
		return ResultNotSet, nil
	}

	var sType []string
	switch tType {
	// NOTE: This make /retest /ok-to-test /test bound to the same policy, which is fine from a security standpoint but maybe we want to refine this in the future.
	case info.TriggerTypeOkToTest, info.TriggerTypeRetest:
		sType = settings.Policy.OkToTest
	case info.TriggerTypePullRequest:
		sType = settings.Policy.PullRequest
	// NOTE: not supported yet, will imp if it gets requested and reasonable to implement
	case info.TriggerTypePush, info.TriggerTypeCancel, info.TriggerTypeCheckSuiteRerequested, info.TriggerTypeCheckRunRerequested:
		return ResultNotSet, nil
	default:
		return ResultNotSet, nil
	}

	if len(sType) == 0 {
		return ResultNotSet, nil
	}

	allowed, reason := p.VCX.CheckPolicyAllowing(ctx, p.Event, sType)
	reasonMsg := fmt.Sprintf("policy check: %s, %s", string(tType), reason)
	if reason != "" {
		p.EventEmitter.EmitMessage(p.Repository, zap.InfoLevel, "PolicyCheck", reasonMsg)
	}
	if allowed {
		return ResultAllowed, nil
	}
	return ResultDisallowed, fmt.Errorf(reasonMsg)
}

func (p *Policy) IsAllowed(ctx context.Context, tType info.TriggerType) (bool, string) {
	var reason string
	policyRes, err := p.checkAllowed(ctx, tType)
	if err != nil {
		return false, err.Error()
	}
	switch policyRes {
	case ResultAllowed:
		reason = fmt.Sprintf("policy check: policy is set for sender %s has been allowed to run CI via policy", p.Event.Sender)
		p.EventEmitter.EmitMessage(p.Repository, zap.InfoLevel, "PolicySetAllowed", reason)
		return true, ""
	case ResultDisallowed:
		reason = fmt.Sprintf("policy check: policy is set but sender %s is not in the allowed groups trying the next ACL conditions", p.Event.Sender)
		p.EventEmitter.EmitMessage(p.Repository, zap.InfoLevel, "PolicySetDisallowed", reason)
	case ResultNotSet:
		// should we put a warning here? it does fill up quite a bit the log every time! so I am not so sure..
	}
	return false, reason
}
