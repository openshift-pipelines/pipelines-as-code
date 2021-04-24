package tektoncli

import (
	"testing"

	clitest "github.com/tektoncd/cli/pkg/test"
	"gotest.tools/assert"
)

func TestPipelineRunDescribe(t *testing.T) {
	cliparams := &clitest.Params{}
	nt, err := New("testns", cliparams)
	assert.NilError(t, err)

	assert.Assert(t, nt.cliOpts.Params.Namespace() == "testns")
	nt.PipelineRunDescribe("hello", "testns")
}
