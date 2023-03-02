package templates

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestReplacePlaceHoldersVariables(t *testing.T) {
	tests := []struct {
		name     string
		template string
		expected string
		dicto    map[string]string
	}{
		{
			name:     "Test Replace",
			template: `revision: {{ revision }}} url: {{ url }} bar: {{ bar}}`,
			expected: `revision: master} url: https://chmouel.com bar: {{ bar}}`,
			dicto: map[string]string{
				"revision": "master",
				"url":      "https://chmouel.com",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ReplacePlaceHoldersVariables(tt.template, tt.dicto)
			if d := cmp.Diff(got, tt.expected); d != "" {
				t.Fatalf("-got, +want: %v", d)
			}
		})
	}
}
