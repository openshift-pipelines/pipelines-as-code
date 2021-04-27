package pipelineascode

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestReplacePlaceHoldersVariables(t *testing.T) {
	template := `revision: {{ revision }}}
url: {{ url }}
bar: {{ bar}}
`
	expected := `revision: master}
url: https://chmouel.com
bar: {{ bar}}
`
	dico := map[string]string{
		"revision": "master",
		"url":      "https://chmouel.com",
	}
	got := ReplacePlaceHoldersVariables(template, dico)
	if d := cmp.Diff(got, expected); d != "" {
		t.Fatalf("-got, +want: %v", d)
	}
}
