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

func TestCancelComment(t *testing.T) {
	tests := []struct {
		name    string
		comment string
		want    bool
	}{
		{
			name:    "valid",
			comment: "/cancel",
			want:    true,
		},
		{
			name:    "valid with some string before",
			comment: "/lgtm \n/cancel",
			want:    true,
		},
		{
			name:    "valid with some string before and after",
			comment: "hi, trigger the ci \n/cancel \n then report the status back",
			want:    true,
		},
		{
			name:    "valid comments",
			comment: "/lgtm \n/cancel \n/approve",
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
		{
			name:    "cancel single pr",
			comment: "/cancel abc",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsCancelComment(tt.comment)
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
			got := GetPipelineRunFromTestComment(tt.comment)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetPipelineRunFromCancelComment(t *testing.T) {
	tests := []struct {
		name    string
		comment string
		want    string
	}{
		{
			name:    "cancel all",
			comment: "/cancel",
			want:    "",
		},
		{
			name:    "cancel a pipeline",
			comment: "/cancel abc-01-pr",
			want:    "abc-01-pr",
		},
		{
			name:    "string before cancel command",
			comment: "abc \n /cancel abc-01-pr",
			want:    "abc-01-pr",
		},
		{
			name:    "string after cancel command",
			comment: "/cancel abc-01-pr \n abc",
			want:    "abc-01-pr",
		},
		{
			name:    "string before and after cancel command",
			comment: "before \n /cancel abc-01-pr \n after",
			want:    "abc-01-pr",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetPipelineRunFromCancelComment(tt.comment)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetPipelineRunAndBranchNameFromTestComment(t *testing.T) {
	tests := []struct {
		name       string
		comment    string
		branchName string
		prName     string
		wantError  bool
	}{
		{
			name:       "retest all on test branch",
			comment:    "/retest branch:test",
			branchName: "test",
			wantError:  false,
		},
		{
			name:       "test a pipeline on test branch",
			comment:    "/test abc-01-pr branch:test",
			prName:     "abc-01-pr",
			branchName: "test",
			wantError:  false,
		},
		{
			name:       "string for test command before branch name test",
			comment:    "/test abc-01-pr abc \n branch:test",
			prName:     "abc-01-pr",
			branchName: "test",
			wantError:  false,
		},
		{
			name:       "string for retest command after branch name test",
			comment:    "/retest abc-01-pr branch:test \n abc",
			prName:     "abc-01-pr",
			branchName: "test",
			wantError:  false,
		},
		{
			name:       "string for test command before and after branch name test",
			comment:    "/test abc-01-pr \n before branch:test \n after",
			prName:     "abc-01-pr",
			branchName: "test",
			wantError:  false,
		},
		{
			name:      "different word other than branch for retest command",
			comment:   "/retest invalidname:nightly",
			wantError: true,
		},
		{
			name:      "test all",
			comment:   "/test",
			wantError: false,
		},
		{
			name:      "test a pipeline",
			comment:   "/test abc-01-pr",
			prName:    "abc-01-pr",
			wantError: false,
		},
		{
			name:      "string before retest command",
			comment:   "abc \n /retest abc-01-pr",
			prName:    "abc-01-pr",
			wantError: false,
		},
		{
			name:      "string after retest command",
			comment:   "/retest abc-01-pr \n abc",
			prName:    "abc-01-pr",
			wantError: false,
		},
		{
			name:      "string before and after test command",
			comment:   "before \n /test abc-01-pr \n after",
			prName:    "abc-01-pr",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prName, branchName, err := GetPipelineRunAndBranchNameFromTestComment(tt.comment)
			assert.Equal(t, tt.wantError, err != nil)
			assert.Equal(t, tt.branchName, branchName)
			assert.Equal(t, tt.prName, prName)
		})
	}
}

func TestGetPipelineRunAndBranchNameFromCancelComment(t *testing.T) {
	tests := []struct {
		name       string
		comment    string
		branchName string
		prName     string
		wantError  bool
	}{
		{
			name:      "cancel all pipeline",
			comment:   "/cancel",
			wantError: false,
		},
		{
			name:      "cancel a particular pipeline",
			comment:   "/cancel abc-01-pr",
			prName:    "abc-01-pr",
			wantError: false,
		},
		{
			name:      "add string before cancel command",
			comment:   "abc \n /cancel abc-01-pr",
			prName:    "abc-01-pr",
			wantError: false,
		},
		{
			name:      "add string after cancel command",
			comment:   "/cancel abc-01-pr \n abc",
			prName:    "abc-01-pr",
			wantError: false,
		},
		{
			name:      "add string before and after cancel command",
			comment:   "before \n /cancel abc-01-pr \n after",
			prName:    "abc-01-pr",
			wantError: false,
		},
		{
			name:       "cancel all on test branch",
			comment:    "/cancel branch:test",
			branchName: "test",
			wantError:  false,
		},
		{
			name:       "cancel a pipeline on test branch",
			comment:    "/cancel abc-01-pr branch:test",
			prName:     "abc-01-pr",
			branchName: "test",
			wantError:  false,
		},
		{
			name:       "string for cancel command before branch name test",
			comment:    "/cancel abc-01-pr abc \n branch:test",
			prName:     "abc-01-pr",
			branchName: "test",
			wantError:  false,
		},
		{
			name:       "string for cancel command after branch name test",
			comment:    "/cancel abc-01-pr branch:test \n abc",
			prName:     "abc-01-pr",
			branchName: "test",
			wantError:  false,
		},
		{
			name:       "string for cancel command before and after branch name test",
			comment:    "/cancel abc-01-pr \n before branch:test \n after",
			prName:     "abc-01-pr",
			branchName: "test",
			wantError:  false,
		},
		{
			name:      "different word other than branch for cancel command",
			comment:   "/cancel invalidname:nightly",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prName, branchName, err := GetPipelineRunAndBranchNameFromCancelComment(tt.comment)
			assert.Equal(t, tt.wantError, err != nil)
			assert.Equal(t, tt.branchName, branchName)
			assert.Equal(t, tt.prName, prName)
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
			name: "exact same",
			url1: "https://shivam.com/foo/bar",
			url2: "https://shivam.com/hello/moto",
			want: true,
		},
		{
			name: "exact same but different",
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
