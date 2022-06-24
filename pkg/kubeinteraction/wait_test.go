package kubeinteraction

import (
	"fmt"
	"testing"
	"time"

	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestPollImmediateWithContext(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
		fn      func() (bool, error)
	}{
		{
			name: "test true",
			fn: func() (bool, error) {
				return true, nil
			},
		},
		{
			name: "test false",
			fn: func() (bool, error) {
				return true, fmt.Errorf("error me")
			},
			wantErr: true,
		},
		{
			name: "timeout",
			fn: func() (bool, error) {
				return false, nil
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			if err := PollImmediateWithContext(ctx, 1*time.Millisecond, tt.fn); (err != nil) != tt.wantErr {
				t.Errorf("PollImmediateWithContext() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
