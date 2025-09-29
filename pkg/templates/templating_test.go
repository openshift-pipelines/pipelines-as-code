package templates

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"gotest.tools/v3/assert"
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
			template: `revision: {{ revision }}} url: {{ url }} bar: {{ bar}} tag: {{ git_tag }}`,
			expected: `revision: master} url: https://chmouel.com bar: {{ bar}} tag: v1.0`,
			dicto: map[string]string{
				"revision": "master",
				"url":      "https://chmouel.com",
				"git_tag":  "v1.0",
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
		{
			name:         "Test with nil rawEvent and headers",
			template:     `body: {{ body.hello }}`,
			expected:     `body: {{ body.hello }}`,
			dicto:        map[string]string{},
			changedFiles: map[string]any{},
			headers:      nil,
			rawEvent:     nil,
		},
		{
			name:         "Test with nil headers only",
			template:     `body: {{ body.hello }}`,
			expected:     `body: world`,
			dicto:        map[string]string{},
			changedFiles: map[string]any{},
			headers:      nil,
			rawEvent: map[string]string{
				"hello": "world",
			},
		},
		{
			name:         "Test CEL with numeric value",
			template:     `count: {{ body.count }}`,
			expected:     `count: 42`,
			dicto:        map[string]string{},
			changedFiles: map[string]any{},
			headers:      http.Header{},
			rawEvent: map[string]any{
				"count": 42,
			},
		},
		{
			name:         "Test CEL with boolean value",
			template:     `enabled: {{ body.enabled }}`,
			expected:     `enabled: true`,
			dicto:        map[string]string{},
			changedFiles: map[string]any{},
			headers:      http.Header{},
			rawEvent: map[string]any{
				"enabled": true,
			},
		},
		{
			name:         "Test CEL with invalid key",
			template:     `invalid: {{ body.nonexistent }}`,
			expected:     `invalid: {{ body.nonexistent }}`,
			dicto:        map[string]string{},
			changedFiles: map[string]any{},
			headers:      http.Header{},
			rawEvent: map[string]any{
				"existing": "value",
			},
		},
		{
			name:     "Test with file prefix but nil rawEvent",
			template: `file: {{ files.modified }}`,
			expected: `file: true`,
			dicto:    map[string]string{},
			changedFiles: map[string]any{
				"modified": "true",
			},
			headers:  http.Header{},
			rawEvent: nil,
		},
		{
			name:         "Test with header prefix but nil rawEvent",
			template:     `header: {{ headers.Accept }}`,
			expected:     `header: application/json`,
			dicto:        map[string]string{},
			changedFiles: map[string]any{},
			headers: http.Header{
				"Accept": []string{"application/json"},
			},
			rawEvent: nil,
		},
		{
			name:     "Test placeholder not found in dico",
			template: `missing: {{ missing_key }}`,
			expected: `missing: {{ missing_key }}`,
			dicto: map[string]string{
				"existing_key": "value",
			},
			changedFiles: map[string]any{},
			headers:      http.Header{},
			rawEvent:     nil,
		},
		{
			name:     "Test multiple placeholders mixed",
			template: `dico: {{ revision }}, body: {{ body.hello }}, header: {{ headers.Test }}`,
			expected: `dico: main, body: world, header: value`,
			dicto: map[string]string{
				"revision": "main",
			},
			changedFiles: map[string]any{},
			headers: http.Header{
				"Test": []string{"value"},
			},
			rawEvent: map[string]any{
				"hello": "world",
			},
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

func TestReplacePlaceHoldersVariablesJSONOutput(t *testing.T) {
	tests := []struct {
		name         string
		template     string
		dicto        map[string]string
		headers      http.Header
		changedFiles map[string]any
		rawEvent     any
		checkFunc    func(t *testing.T, result string)
	}{
		{
			name:         "CEL array serialization",
			template:     `items: {{ body.items }}`,
			dicto:        map[string]string{},
			changedFiles: map[string]any{},
			headers:      http.Header{},
			rawEvent: map[string]any{
				"items": []string{"item1", "item2"},
			},
			checkFunc: func(t *testing.T, result string) {
				assert.Assert(t, strings.Contains(result, `"item1"`), "should contain item1")
				assert.Assert(t, strings.Contains(result, `"item2"`), "should contain item2")
				assert.Assert(t, strings.HasPrefix(result, "items: ["), "should start with 'items: ['")
				assert.Assert(t, strings.HasSuffix(result, "]"), "should end with ']'")
			},
		},
		{
			name:         "CEL object serialization",
			template:     `config: {{ body.config }}`,
			dicto:        map[string]string{},
			changedFiles: map[string]any{},
			headers:      http.Header{},
			rawEvent: map[string]any{
				"config": map[string]any{
					"name":  "test",
					"value": "123",
				},
			},
			checkFunc: func(t *testing.T, result string) {
				assert.Assert(t, strings.Contains(result, `"name"`), "should contain name key")
				assert.Assert(t, strings.Contains(result, `"test"`), "should contain test value")
				assert.Assert(t, strings.Contains(result, `"value"`), "should contain value key")
				assert.Assert(t, strings.Contains(result, `"123"`), "should contain 123 value")
				assert.Assert(t, strings.HasPrefix(result, "config: {"), "should start with 'config: {'")
				assert.Assert(t, strings.HasSuffix(result, "}"), "should end with '}'")
			},
		},
		{
			name:         "CEL double value",
			template:     `price: {{ body.price }}`,
			dicto:        map[string]string{},
			changedFiles: map[string]any{},
			headers:      http.Header{},
			rawEvent: map[string]any{
				"price": 42.5,
			},
			checkFunc: func(t *testing.T, result string) {
				assert.Assert(t, strings.Contains(result, "42.5"), "should contain double value")
				assert.Assert(t, strings.HasPrefix(result, "price: "), "should start with 'price: '")
			},
		},
		{
			name:         "CEL int value",
			template:     `age: {{ body.age }}`,
			dicto:        map[string]string{},
			changedFiles: map[string]any{},
			headers:      http.Header{},
			rawEvent: map[string]any{
				"age": int64(25),
			},
			checkFunc: func(t *testing.T, result string) {
				assert.Assert(t, strings.Contains(result, "25"), "should contain int value")
				assert.Assert(t, strings.HasPrefix(result, "age: "), "should start with 'age: '")
			},
		},
		{
			name:         "CEL bytes value",
			template:     `data: {{ body.data }}`,
			dicto:        map[string]string{},
			changedFiles: map[string]any{},
			headers:      http.Header{},
			rawEvent: map[string]any{
				"data": []byte("hello"),
			},
			checkFunc: func(t *testing.T, result string) {
				assert.Assert(t, strings.HasPrefix(result, "data: "), "should start with 'data: '")
				// Bytes might be base64 encoded or handled differently
				assert.Assert(t, len(result) > len("data: "), "should have some data content")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ReplacePlaceHoldersVariables(tt.template, tt.dicto, tt.rawEvent, tt.headers, tt.changedFiles)
			tt.checkFunc(t, got)
		})
	}
}

func TestReplacePlaceHoldersVariablesEdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		template     string
		dicto        map[string]string
		headers      http.Header
		changedFiles map[string]any
		rawEvent     any
		expected     string
	}{
		{
			name:         "CEL expression with complex nested access",
			template:     `nested: {{ body.deep.nested.value }}`,
			dicto:        map[string]string{},
			changedFiles: map[string]any{},
			headers:      http.Header{},
			rawEvent: map[string]any{
				"deep": map[string]any{
					"nested": map[string]any{
						"value": "found",
					},
				},
			},
			expected: `nested: found`,
		},
		{
			name:         "CEL with nil in nested structure",
			template:     `null: {{ body.value }}`,
			dicto:        map[string]string{},
			changedFiles: map[string]any{},
			headers:      http.Header{},
			rawEvent: map[string]any{
				"value": nil,
			},
			expected: `null: `,
		},
		{
			name:         "Multi-level headers access",
			template:     `user-agent: {{ headers["User-Agent"] }}`,
			dicto:        map[string]string{},
			changedFiles: map[string]any{},
			headers: http.Header{
				"User-Agent": []string{"test-agent/1.0"},
			},
			rawEvent: map[string]any{},
			expected: `user-agent: test-agent/1.0`,
		},
		{
			name:     "Files with nested structure",
			template: `file-info: {{ files.metadata.size }}`,
			dicto:    map[string]string{},
			changedFiles: map[string]any{
				"metadata": map[string]any{
					"size": 1024,
				},
			},
			headers:  http.Header{},
			rawEvent: nil,
			expected: `file-info: 1024`,
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
