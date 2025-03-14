package templates

import (
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestReplacePlaceHoldersVariables(t *testing.T) {
	tests := []struct {
		name         string
		template     string
		expected     string
		dicto        map[string]string
		headers      http.Header
		changedFiles map[string]any
		rawEvent     any
	}{
		{
			name:     "Test Replace standard",
			template: `revision: {{ revision }}} url: {{ url }} bar: {{ bar}}`,
			expected: `revision: master} url: https://chmouel.com bar: {{ bar}}`,
			dicto: map[string]string{
				"revision": "master",
				"url":      "https://chmouel.com",
			},
			changedFiles: map[string]any{},
		},
		{
			name:         "Test Replace with CEL body",
			template:     `hello: {{ body.hello }}`,
			expected:     `hello: world`,
			dicto:        map[string]string{},
			changedFiles: map[string]any{},
			headers:      http.Header{},
			rawEvent: map[string]string{
				"hello": "world",
			},
		},
		{
			name:         "Test Replace with CEL body expression",
			template:     `Is this {{ body.hello == 'world' }}`,
			expected:     `Is this true`,
			dicto:        map[string]string{},
			changedFiles: map[string]any{},
			headers:      http.Header{},
			rawEvent: map[string]string{
				"hello": "world",
			},
		},
		{
			name:         "Test Replace with headers",
			template:     `header: {{ headers["X-Hello"] }}`,
			expected:     `header: World`,
			dicto:        map[string]string{},
			changedFiles: map[string]any{},
			headers: http.Header{
				"X-Hello": []string{"World"},
			},
			rawEvent: map[string]string{},
		},
		{
			name:     "Changed files - changed",
			template: `changed: {{ files.all[0] }}`,
			expected: `changed: changed.txt`,
			dicto:    map[string]string{},
			changedFiles: map[string]any{
				"all": []string{"changed.txt"},
			},
			headers:  http.Header{},
			rawEvent: map[string]string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ReplacePlaceHoldersVariables(tt.template, tt.dicto, tt.rawEvent, tt.headers, tt.changedFiles)
			if d := cmp.Diff(got, tt.expected); d != "" {
				t.Fatalf("-got, +want: %v", d)
			}
		})
	}
}
