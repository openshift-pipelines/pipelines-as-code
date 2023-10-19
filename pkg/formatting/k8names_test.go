package formatting

import (
	"testing"
)

func TestCleanKubernetesName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "keep dash",
			input: "foo-bar",
			want:  "foo-bar",
		},
		{
			name:  "keep dot",
			input: "foo.bar",
			want:  "foo.bar",
		},
		{
			name:  "start with lowercase",
			input: "foo",
			want:  "foo",
		},
		{
			name:  "start with uppercase",
			input: "Foo",
			want:  "foo",
		},
		{
			name:  "start with an alphanumeric character",
			input: "foo",
			want:  "foo",
		},
		{
			name:  "end with an alphanumeric character",
			input: "foo",
			want:  "foo",
		},
		{
			name:  "start with special character",
			input: "!foo",
			want:  "-foo",
		},
		{
			name:  "end with special character",
			input: "foo!",
			want:  "foo-",
		},
		{
			name:  "replace slash",
			input: "foo/bar",
			want:  "foo-bar",
		},
		{
			name:  "replace angle bracket",
			input: "foo<bar",
			want:  "foo-bar",
		},
		{
			name:  "replace square bracket",
			input: "foo[bar",
			want:  "foo-bar",
		},
		{
			name:  "replace percent",
			input: "foo%bar",
			want:  "foo-bar",
		},
		{
			name:  "replace new line",
			input: "foo\n",
			want:  "foo",
		},
		{
			name:  "contains spaces",
			input: "foo bar",
			want:  "foo-bar",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CleanKubernetesName(tt.input); got != tt.want {
				t.Errorf("CleanKubernetesName() = %v, want %v", got, tt.want)
			}
		})
	}
}
