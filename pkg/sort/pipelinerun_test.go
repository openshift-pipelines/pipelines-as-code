package sort

import (
	"testing"

	"github.com/jonboulle/clockwork"
	tektontest "github.com/openshift-pipelines/pipelines-as-code/pkg/test/tekton"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"gotest.tools/v3/assert"
)

func TestPipelineRunSortByCompletionTime(t *testing.T) {
	clock := clockwork.NewFakeClock()
	ns := "namespace"
	labels := map[string]string{}
	success := v1beta1.PipelineRunReasonSuccessful.String()
	tests := []struct {
		name    string
		pruns   []v1beta1.PipelineRun
		wantPRs []string
	}{
		{
			pruns: []v1beta1.PipelineRun{
				*(tektontest.MakePRCompletion(clock, "troisieme", ns, success, labels, 30)),
				*(tektontest.MakePRCompletion(clock, "premier", ns, success, labels, 10)),
				*(tektontest.MakePRCompletion(clock, "second", ns, success, labels, 20)),
			},
			wantPRs: []string{"premier", "second", "troisieme"},
		},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for key, value := range PipelineRunSortByCompletionTime(tt.pruns) {
				assert.Equal(t, tt.wantPRs[key], value.GetName())
			}
		})
	}
}
