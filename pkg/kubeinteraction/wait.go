package kubeinteraction

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	interval = 1 * time.Second
)

func PollImmediateWithContext(ctx context.Context, pollTimeout time.Duration, fn func() (bool, error)) error {
	//nolint: staticcheck
	return wait.PollImmediate(interval, pollTimeout, func() (bool, error) {
		select {
		case <-ctx.Done():
			return true, fmt.Errorf("polling timed out, pipelinerun has exceeded its timeout: %v", pollTimeout)
		default:
		}
		return fn()
	})
}
