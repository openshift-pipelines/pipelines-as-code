package pipelineascode

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestPacRun_checkNeedUpdate(t *testing.T) {
	tests := []struct {
		name                 string
		tmpl                 string
		upgradeMessageSubstr string
		needupdate           bool
	}{
		{
			name: "old secrets",
			tmpl: `		  secretName: "pac-git-basic-auth-{{repo_owner}}-{{repo_name}}"`,
			upgradeMessageSubstr: "old basic auth secret name",
			needupdate:           true,
		},
		{
			name:       "no need",
			tmpl:       ` secretName: "foo-bar-foo"`,
			needupdate: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPacs(nil, nil, nil, nil, nil)
			got, needupdate := p.checkNeedUpdate(tt.tmpl)
			if tt.upgradeMessageSubstr != "" {
				assert.Assert(t, strings.Contains(got, tt.upgradeMessageSubstr))
			}
			assert.Assert(t, needupdate == tt.needupdate)
		})
	}
}
