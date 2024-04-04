package opscomments

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestParseKeyValueArgs(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  map[string]string
	}{
		{
			name:  "do not start with /",
			input: "test foo key=value",
			want:  nil,
		},

		{
			name:  "parse simple",
			input: "/test foo key=value",
			want:  map[string]string{"key": "value"},
		},
		{
			name:  "parse quoted with space",
			input: `/test foo key="value"`,
			want:  map[string]string{"key": "value"},
		},
		{
			name:  "parse multiple mix",
			input: `/test foo key="value" another="value"`,
			want:  map[string]string{"key": "value", "another": "value"},
		},
		{
			name:  "parse multiple mix with non proper keyvalue",
			input: `/test foo key="value" another="value" hello="moto`,
			want:  map[string]string{"key": "value", "another": "value"},
		},
		{
			name: "parse with newline",
			input: `/test foo the="value

is value" key=blah`,
			want: map[string]string{"the": "value\n\nis value", "key": "blah"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.DeepEqual(t, ParseKeyValueArgs(tt.input), tt.want)
		})
	}
}
