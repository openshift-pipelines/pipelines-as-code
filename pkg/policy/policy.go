package policy

import (
	"context"
	"fmt"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
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
	Settings *v1alpha1.Settings
	Event    *info.Event
	VCX      provider.Interface
	Logger   *zap.SugaredLogger
}

func (p *Policy) IsAllowed(ctx context.Context, tType info.TriggerType) (Result, error) {
	if p.Settings == nil || p.Settings.Policy == nil {
		return ResultNotSet, nil
	}

	var sType []string
	switch tType {
	// NOTE: This make /retest /ok-to-test /test bound to the same policy, which is fine from a security standpoint but maybe we want to refind
	case info.TriggerTypeOkToTest, info.TriggerTypeRetest:
		sType = p.Settings.Policy.OkToTest
	case info.TriggerTypePullRequest:
		sType = p.Settings.Policy.PullRequest
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
		p.Logger.Info(reasonMsg)
	}
	if allowed {
		return ResultAllowed, nil
	}
	return ResultDisallowed, fmt.Errorf(reasonMsg)
}
