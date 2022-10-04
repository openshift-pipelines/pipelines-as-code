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

func TestIsTestRetestComment(t *testing.T) {
	tests := []struct {
		name    string
		comment string
		want    bool
	}{
		{
			name:    "valid retest",
			comment: "/retest",
			want:    true,
		},
		{
			name:    "valid test",
			comment: "/test",
			want:    true,
		},
		{
			name:    "valid retest with some string before",
			comment: "/lgtm \n/retest",
			want:    true,
		},
		{
			name:    "valid test with some string before",
			comment: "/lgtm \n/test",
			want:    true,
		},
		{
			name:    "valid with some string before and after",
			comment: "hi, trigger the ci \n/retest \n then report the status back",
			want:    true,
		},
		{
			name:    "test valid with some string before and after",
			comment: "hi, trigger the ci \n/test \n then report the status back",
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
			name:    "retest trigger single pr",
			comment: "/retest abc",
			want:    true,
		},
		{
			name:    "test trigger single pr",
			comment: "/test abc",
			want:    true,
		},
		{
			name:    "invalid",
			comment: "test abc",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsTestRetestComment(tt.comment)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetPipelineRunFromComment(t *testing.T) {
	tests := []struct {
		name    string
		comment string
		want    string
	}{
		{
			name:    "test no pipelinerun",
			comment: "/test",
			want:    "",
		},
		{
			name:    "retest no pipelinerun",
			comment: "/retest",
			want:    "",
		},
		{
			name:    "test a pipeline",
			comment: "/test abc-01-pr",
			want:    "abc-01-pr",
		},
		{
			name:    "string before test command",
			comment: "abc \n /test abc-01-pr",
			want:    "abc-01-pr",
		},
		{
			name:    "string after test command",
			comment: "/test abc-01-pr \n abc",
			want:    "abc-01-pr",
		},
		{
			name:    "string before and after test command",
			comment: "before \n /test abc-01-pr \n after",
			want:    "abc-01-pr",
		},
		{
			name:    "retest a pipeline",
			comment: "/retest abc-01-pr",
			want:    "abc-01-pr",
		},
		{
			name:    "string before retest command",
			comment: "abc \n /retest abc-01-pr",
			want:    "abc-01-pr",
		},
		{
			name:    "string after retest command",
			comment: "/retest abc-01-pr \n abc",
			want:    "abc-01-pr",
		},
		{
			name:    "string before and after retest command",
			comment: "before \n /retest abc-01-pr \n after",
			want:    "abc-01-pr",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetPipelineRunFromComment(tt.comment)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCompareHostOfURLS(t *testing.T) {
	tests := []struct {
		name string
		url1 string
		url2 string
		want bool
	}{
		{
			name: "same same",
			url1: "https://shivam.com/foo/bar",
			url2: "https://shivam.com/hello/moto",
			want: true,
		},
		{
			name: "same same but different",
			url1: "https://shivam.com/foo/bar",
			url2: "https://vincent.com/foo/bar",
			want: false,
		},
		{
			name: "bad url1",
			url1: "i am such a bad url",
			want: false,
		},
		{
			name: "bad url2",
			url2: "i am the baddest, choose me!",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CompareHostOfURLS(tt.url1, tt.url2)
			assert.Equal(t, tt.want, got)
		})
	}
}
