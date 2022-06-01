package repository

import (
	"bytes"
	"strings"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	tcli "github.com/openshift-pipelines/pipelines-as-code/pkg/test/cli"
	"gotest.tools/v3/assert"
)

func TestRoot(t *testing.T) {
	buf := &bytes.Buffer{}
	s, err := tcli.ExecuteCommand(Root(&params.Run{}, &cli.IOStreams{Out: buf, ErrOut: buf}), "help")
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(s, "repository, repo, repositories"))
}
