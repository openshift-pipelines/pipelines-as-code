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
			name:    "valid with some string before",
			comment: "/lgtm \n/ok-to-test",
			want:    true,
		},
		{
			name:    "valid with some string before and after",
			comment: "hi, trigger the ci \n/ok-to-test \n then report the status back",
			want:    true,
		},
		{
			name:    "valid comments",
			comment: "/lgtm \n/ok-to-test \n/approve",
			want:    true,
		},
		{
			name:    "invalid",
			comment: "/ok",
			want:    false,
		},
		{
			name:    "invalid comment",
			comment: "/ok-to-test abc",
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
			name:    "valid with some string before",
			comment: "/lgtm \n/retest",
			want:    true,
		},
		{
			name:    "valid with some string before and after",
			comment: "hi, trigger the ci \n/retest \n then report the status back",
			want:    true,
		},
		{
			name:    "valid comments",
			comment: "/lgtm \n/retest \n/approve",
			want:    true,
		},
		{
			name:    "invalid",
			comment: "/ok",
			want:    false,
		},
		{
			name:    "invalid comment",
			comment: "/retest abc",
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
