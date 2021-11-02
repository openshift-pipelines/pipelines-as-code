package consoleui

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestTektonDashboard(t *testing.T) {
	tr := &TektonDashboard{
		BaseURL: "https://test",
	}
	assert.Assert(t, strings.Contains(tr.DetailURL("ns", "pr"), "namespaces/ns"))
	assert.Assert(t, strings.Contains(tr.TaskLogURL("ns", "pr", "task"), "pipelineTask=task"))
	assert.Assert(t, strings.Contains(tr.URL(), "test"))
}
