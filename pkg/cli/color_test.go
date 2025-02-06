package cli

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestColorScheme(t *testing.T) {
	// Enable colors for testing
	t.Setenv("CLICOLOR_FORCE", "1")

	cs := NewColorScheme(true, true)

	tests := []struct {
		name     string
		function func(string) string
		input    string
		expected string
	}{
		{"Red", cs.Red, "test", red("test")},
		{"Green", cs.Green, "test", green("test")},
		{"Blue", cs.Blue, "test", blue("test")},
		{"Yellow", cs.Yellow, "test", yellow("test")},
		{"Magenta", cs.Magenta, "test", magenta("test")},
		{"Cyan", cs.Cyan, "test", cyan("test")},
		{"Gray", cs.Gray, "test", gray256("test")},
		{"Bold", cs.Bold, "test", bold("test")},
		{"Underline", cs.Underline, "test", underline("test")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.function(tt.input)
			assert.Equal(t, got, tt.expected, "ColorScheme.%s() = %v, want %v", tt.name, got, tt.expected)
		})
	}
}

func TestColorStatus(t *testing.T) {
	cs := NewColorScheme(true, true)

	tests := []struct {
		status   string
		expected string
	}{
		{"succeeded", cs.Green("succeeded")},
		{"failed", cs.Red("failed")},
		{"pipelineruntimeout", cs.Yellow("Timeout")},
		{"norun", cs.Dimmed("norun")},
		{"running", cs.Blue("running")},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := cs.ColorStatus(tt.status)
			assert.Equal(t, got, tt.expected, "ColorScheme.ColorStatus() = %v, want %v", got, tt.expected)
		})
	}
}
