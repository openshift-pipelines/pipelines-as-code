package sort

import (
	"regexp"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/consoleui"
	tektontest "github.com/openshift-pipelines/pipelines-as-code/pkg/test/tekton"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"gotest.tools/v3/assert"
)

func TestStatusTmpl(t *testing.T) {
	flattedTmpl := `{{- range $taskrun := .TaskRunList }}{{- $taskrun.ConsoleLogURL }}{{- end }}`

	tests := []struct {
		name       string
		wantErr    bool
		pr         *tektonv1beta1.PipelineRun
		tmpl       string
		wantRegexp *regexp.Regexp
	}{
		{
			name:    "badtemplate",
			wantErr: true,
			tmpl:    "{{.XXX}}",
			pr: tektontest.MakePR("pr1", "ns1", map[string]*tektonv1beta1.PipelineRunTaskRunStatus{
				"first":  tektontest.MakePrTrStatus("first", 5),
				"last":   tektontest.MakePrTrStatus("last", 15),
				"middle": tektontest.MakePrTrStatus("middle", 10),
			}, nil),
		},
		{
			name:       "sorted",
			wantRegexp: regexp.MustCompile(".*first.*middle.*last.*"),
			tmpl:       flattedTmpl,
			pr: tektontest.MakePR("pr1", "ns1", map[string]*tektonv1beta1.PipelineRunTaskRunStatus{
				"first":  tektontest.MakePrTrStatus("first", 5),
				"last":   tektontest.MakePrTrStatus("last", 15),
				"middle": tektontest.MakePrTrStatus("middle", 10),
			}, nil),
		},
		{
			name:       "not completed yet come first",
			wantRegexp: regexp.MustCompile(".*notcompleted.*first.*last.*"),
			tmpl:       flattedTmpl,
			pr: tektontest.MakePR("pr1", "ns1", map[string]*tektonv1beta1.PipelineRunTaskRunStatus{
				"first":  tektontest.MakePrTrStatus("first", 5),
				"last":   tektontest.MakePrTrStatus("last", 15),
				"middle": tektontest.MakePrTrStatus("notcompleted", -1),
			}, nil),
		},
		{
			name:       "test sorted status nada",
			wantRegexp: regexp.MustCompile("No tasks has been found"),
			pr:         tektontest.MakePR("nada", "ns", nil, nil),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			console := consoleui.FallBackConsole{}
			output, err := TaskStatusTmpl(tt.pr, console, tt.tmpl)
			if tt.wantErr {
				assert.Assert(t, err != nil)
				return
			}
			assert.NilError(t, err)
			if tt.wantRegexp != nil {
				assert.Assert(t, tt.wantRegexp.MatchString(output), "%s != %s", output, tt.wantRegexp.String())
			}
		})
	}
}
