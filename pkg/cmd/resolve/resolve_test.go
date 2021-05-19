package resolve

import (
	"fmt"
	"strings"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	assertfs "gotest.tools/v3/fs"
)

var tmplSimpleNoPrefix = `
apiVersion: tekton.dev/v1beta1
kind: PipelineRun
metadata:
  name: test
spec:
  pipelineSpec:
	tasks:
	  - name: hello1
		taskSpec:
		  steps:
			- name: hello-moto
			  image: alpine:3.7
			  script: "echo hello moto"
`

func TestResolveFilenames(t *testing.T) {
	observer, _ := zapobserver.New(zap.InfoLevel)
	fakelogger := zap.New(observer).Sugar()
	cs := &cli.Clients{Log: fakelogger}

	tmplSimpleWithPrefix := fmt.Sprintf("---\n%s", tmplSimpleNoPrefix)

	tests := []struct {
		name    string
		tmpl    string
		wantErr bool
	}{
		{
			name:    "Resolve templates no prefix",
			tmpl:    tmplSimpleNoPrefix,
			wantErr: false,
		},
		{
			name:    "Resolve templates with prefix",
			tmpl:    tmplSimpleWithPrefix,
			wantErr: false,
		},
		{
			name:    "No pipelinerun",
			tmpl:    `---\nfoo:bar`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := assertfs.NewDir(t, "test-name",
				assertfs.WithFile("file.yaml", strings.ReplaceAll(tt.tmpl, "\t", "    ")))
			defer dir.Remove()
			got, err := resolveFilenames(cs, []string{dir.Path()})
			if (err != nil) != tt.wantErr {
				t.Errorf("resolveFilenames() error = %v, wantErr %v", err, tt.wantErr)
				return
			} else if !tt.wantErr {
				assert.Assert(t, got != "")
			}
		})
	}
}
