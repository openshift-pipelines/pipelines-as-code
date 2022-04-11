package kubeinteraction

import (
	"context"
	"fmt"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	tektonv1beta1client "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/typed/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	knativeapi "knative.dev/pkg/apis"
)

const (
	interval = 2 * time.Second
)

type ConditionAccessorFn func(ca knativeapi.ConditionAccessor) (bool, error)

// Running provides a poll condition function that checks if the ConditionAccessor
// resource is currently running.
func Running(name string) ConditionAccessorFn {
	return func(ca knativeapi.ConditionAccessor) (bool, error) {
		c := ca.GetCondition(knativeapi.ConditionSucceeded)
		if c != nil {
			if c.Status == corev1.ConditionTrue || c.Status == corev1.ConditionFalse {
				return true, fmt.Errorf(`%q already finished`, name)
			} else if c.Status == corev1.ConditionUnknown && (c.Reason == "Running" || c.Reason == "Pending") {
				return true, nil
			}
		}
		return false, nil
	}
}

// PipelineRunPending provides a poll condition function that checks if the PipelineRun
// has been marked pending by the Tekton controller.
func PipelineRunPending(name string) ConditionAccessorFn {
	running := Running(name)

	return func(ca knativeapi.ConditionAccessor) (bool, error) {
		c := ca.GetCondition(knativeapi.ConditionSucceeded)
		if c != nil {
			if c.Status == corev1.ConditionUnknown && c.Reason == string(v1beta1.PipelineRunReasonPending) {
				return true, nil
			}
		}
		status, err := running(ca)
		if status {
			reason := ""
			// c _should_ never be nil if we get here, but we have this check just in case.
			if c != nil {
				reason = c.Reason
			}
			return false, fmt.Errorf("status should be %s, but it is %s", v1beta1.PipelineRunReasonPending, reason)
		}
		return status, err
	}
}

// Succeed provides a poll condition function that checks if the ConditionAccessor
// resource has successfully completed or not.
func Succeed(name string) ConditionAccessorFn {
	return func(ca knativeapi.ConditionAccessor) (bool, error) {
		c := ca.GetCondition(knativeapi.ConditionSucceeded)
		if c != nil {
			if c.Status == corev1.ConditionTrue {
				return true, nil
			} else if c.Status == corev1.ConditionFalse {
				return true, fmt.Errorf("%q failed", name)
			}
		}
		return false, nil
	}
}

// PipelineRunSucceed provides a poll condition function that checks if the PipelineRun
// has successfully completed.
func PipelineRunSucceed(name string) ConditionAccessorFn {
	return Succeed(name)
}

func PollImmediateWithContext(ctx context.Context, pollTimeout time.Duration, fn func() (bool, error)) error {
	return wait.PollImmediate(interval, pollTimeout, func() (bool, error) {
		select {
		case <-ctx.Done():
			return true, fmt.Errorf("polling timed out, pipelinerun has exceeded its timeout: %v", pollTimeout)
		default:
		}
		return fn()
	})
}

// WaitForPipelineRunState polls the status of the PipelineRun called name from client every
// interval until inState returns `true` indicating it is done, returns an
// error or timeout. desc will be used to name the metric that is emitted to
// track how long it took for name to get into the state checked by inState.
func waitForPipelineRunState(ctx context.Context, tektonbeta1 tektonv1beta1client.TektonV1beta1Interface, pr *v1beta1.PipelineRun, polltimeout time.Duration, inState ConditionAccessorFn) error {
	ctx, cancel := context.WithTimeout(ctx, polltimeout)
	defer cancel()
	return PollImmediateWithContext(ctx, polltimeout, func() (bool, error) {
		r, err := tektonbeta1.PipelineRuns(pr.Namespace).Get(ctx, pr.Name, metav1.GetOptions{})
		if err != nil {
			return true, err
		}
		return inState(&r.Status)
	})
}

func (k Interaction) WaitForPipelineRunSucceed(ctx context.Context, tektonbeta1 tektonv1beta1client.TektonV1beta1Interface, pr *v1beta1.PipelineRun, polltimeout time.Duration) error {
	return waitForPipelineRunState(ctx, tektonbeta1, pr, polltimeout, PipelineRunSucceed(pr.Name))
}
