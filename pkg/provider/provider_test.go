package provider

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestIsOkToTestComment(t *testing.T) {
	tests := []struct {
		name    string
		comment string
		want    bool
	}{
		{
			name:    "valid",
			comment: "/ok-to-test",
			want:    true,
		},
		{
			name:    "invalid",
			comment: "/ok",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsOkToTestComment(tt.comment)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsRetestComment(t *testing.T) {
	tests := []struct {
		name    string
		comment string
		want    bool
	}{
		{
			name:    "valid",
			comment: "/retest",
			want:    true,
		},
		{
			name:    "invalid",
			comment: "/ok",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsRetestComment(tt.comment)
			assert.Equal(t, tt.want, got)
		})
	}
}
