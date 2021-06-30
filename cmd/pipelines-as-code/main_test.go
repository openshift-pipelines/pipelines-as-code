package main

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestRouteBinary(t *testing.T) {
	tests := []struct {
		name  string
		short string
	}{
		{
			name:  "pipelines-as-code",
			short: "Pipelines as code Run",
		},
		{
			name:  "tkn-pac",
			short: "Pipelines as Code CLI",
		},
		{
			name:  "anything-else",
			short: "Pipelines as Code CLI",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, RouteBinary(tt.name).Short, tt.short)
		})
	}
}
