package repository

import (
	"strings"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	tcli "github.com/openshift-pipelines/pipelines-as-code/pkg/test/cli"
	"gotest.tools/v3/assert"
)

func TestRoot(t *testing.T) {
	s, err := tcli.ExecuteCommand(Root(&cli.PacParams{}), "help")
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(s, "repository, repo, repsitories"))
}
