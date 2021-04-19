package pipelineascode

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/test"
	"gotest.tools/assert"
)

func TestProcessTektonYaml(t *testing.T) {
	data := `tasks:
- foo
- bar:latest
- https://foo.bar
- https://hello.moto
`
	cs := test.MakeHttpTestClient(t, 200, "HELLO")
	expected := `---
HELLO
---
HELLO`

	ret, err := processTektonYaml(cs, data)
	assert.NilError(t, err)
	if d := cmp.Diff(ret.RemoteTasks, expected); d != "" {
		t.Fatalf("-got, +want: %v", d)
	}

}
