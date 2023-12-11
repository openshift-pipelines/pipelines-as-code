package webhook

import (
	"context"
	"testing"
)

// Test_Reconcile tests the reconcile function
// TODO: make it a more complete test.
func Test_Reconcile(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "run reconcile",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ac := &reconciler{}
			if err := ac.Reconcile(context.Background(), ""); (err != nil) != tt.wantErr {
				t.Errorf("reconciler.Reconcile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
