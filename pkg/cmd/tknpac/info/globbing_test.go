package info

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tknpactest "github.com/openshift-pipelines/pipelines-as-code/test/pkg/cli"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"
	"gotest.tools/v3/golden"
)

func TestGlobbing(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		wantErr bool
		files   []string
		str     string
	}{
		{
			name:    "File Globbing",
			pattern: "***/*.md",
			files: []string{
				"README.md",
				"docs/blah.md",
				"hello/moto.md",
			},
		},
		{
			name:    "String Pattern",
			pattern: "refs/*/release-*",
			str:     "refs/heads/release-foo",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.str != "" {
				output, err := tknpactest.ExecCommandNoRun(globbingCommand, "-s", tt.str, tt.pattern)
				if tt.wantErr {
					assert.Assert(t, tt.wantErr, err != nil, "wantErr: %v, got: %v", tt.wantErr, err)
					return
				}
				golden.Assert(t, output, strings.ReplaceAll(fmt.Sprintf("%s.golden", t.Name()), "/", "-"))
				return
			}

			tmpdir := fs.NewDir(t, t.Name())
			defer tmpdir.Remove()
			for _, file := range tt.files {
				assert.NilError(t, os.MkdirAll(filepath.Dir(filepath.Join(tmpdir.Path(), file)), 0o755))
				f, err := os.Create(filepath.Join(tmpdir.Path(), file))
				assert.NilError(t, err)
				_, _ = f.WriteString("")
				f.Close()
			}
			output, err := tknpactest.ExecCommandNoRun(globbingCommand, "-d", tmpdir.Path(), tt.pattern)
			assert.NilError(t, err)
			golden.Assert(t, output, strings.ReplaceAll(fmt.Sprintf("%s.golden", t.Name()), "/", "-"))
		})
	}
}
