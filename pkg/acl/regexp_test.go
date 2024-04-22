package acl

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestRegexp(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		matched bool
	}{
		{
			name:    "bad/match regexp",
			text:    "foo bar",
			matched: false,
		},
		{
			name:    "good/match regexp",
			text:    "/ok-to-test",
			matched: true,
		},
		{
			name:    "good/match regexp newline",
			text:    "\n/ok-to-test",
			matched: true,
		},
		{
			name:    "bad/match regexp newline space",
			text:    "\n /ok-to-test",
			matched: false,
		},
		{
			name:    "good/in the middle",
			text:    "foo bar\n/ok-to-test\nhello moto",
			matched: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched := MatchRegexp(OKToTestCommentRegexp, tt.text)
			assert.Equal(t, tt.matched, matched)
		})
	}
}

func TestMatchRegexp(t *testing.T) {
	type args struct {
		reg     string
		comment string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "match",
			args: args{
				reg:     ".*",
				comment: "hello",
			},
			want: true,
		},
		{
			name: "nomatch",
			args: args{
				reg:     "!!!",
				comment: "foobar",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchRegexp(tt.args.reg, tt.args.comment); got != tt.want {
				t.Errorf("MatchRegexp() = %v, want %v", got, tt.want)
			}
		})
	}
}
