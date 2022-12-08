package sort

import (
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	tektontest "github.com/openshift-pipelines/pipelines-as-code/pkg/test/tekton"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPipelineRunSortByCompletionTime(t *testing.T) {
	clock := clockwork.NewFakeClock()
	ns := "namespace"
	labels := map[string]string{}
	success := v1beta1.PipelineRunReasonSuccessful.String()
	tests := []struct {
		name     string
		pruns    []v1beta1.PipelineRun
		wantName []string
	}{
		{
			pruns: []v1beta1.PipelineRun{
				*(tektontest.MakePRCompletion(clock, "troisieme", ns, success, labels, 30)),
				*(tektontest.MakePRCompletion(clock, "premier", ns, success, labels, 10)),
				*(tektontest.MakePRCompletion(clock, "second", ns, success, labels, 20)),
			},
			wantName: []string{"premier", "second", "troisieme"},
		},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for key, value := range PipelineRunSortByCompletionTime(tt.pruns) {
				assert.Equal(t, tt.wantName[key], value.GetName())
			}
		})
	}
}

func TestPipelineRunSortByStartTime(t *testing.T) {
	clock := clockwork.NewFakeClock()
	ns := "namespace"
	labels := map[string]string{}
	success := v1beta1.PipelineRunReasonSuccessful.String()
	startedEarlierPR := tektontest.MakePRCompletion(clock, "earlier", ns, success, labels, 5)
	startedEarlierPR.Status.StartTime = &metav1.Time{Time: clock.Now().Add(100 * time.Minute)}

	noCompletionPR := tektontest.MakePRCompletion(clock, "noCompletion", ns, success, labels, 5)
	noCompletionPR.Status.StartTime = &metav1.Time{Time: clock.Now().Add(500 * time.Minute)}
	noCompletionPR.Status.CompletionTime = nil

	notStartedYet := tektontest.MakePRCompletion(clock, "notStarted", ns, success, labels, 5)
	noCompletionPR.Status.StartTime = nil
	noCompletionPR.Status.CompletionTime = nil

	tests := []struct {
		name     string
		pruns    []v1beta1.PipelineRun
		wantName []string
	}{
		{
			name: "finished last started first",
			pruns: []v1beta1.PipelineRun{
				*(tektontest.MakePRCompletion(clock, "otherFirst", ns, success, labels, 30)),
				*(tektontest.MakePRCompletion(clock, "otherSecond", ns, success, labels, 10)),
				*startedEarlierPR,
			},
			wantName: []string{"earlier", "otherFirst", "otherSecond"},
		},
		{
			name: "no completion but started first",
			pruns: []v1beta1.PipelineRun{
				*noCompletionPR,
				*(tektontest.MakePRCompletion(clock, "otherFirst", ns, success, labels, 30)),
				*(tektontest.MakePRCompletion(clock, "otherSecond", ns, success, labels, 10)),
			},
			wantName: []string{"noCompletion", "otherFirst", "otherSecond"},
		},

		{
			name: "not started yet",
			pruns: []v1beta1.PipelineRun{
				*notStartedYet,
				*(tektontest.MakePRCompletion(clock, "otherFirst", ns, success, labels, 30)),
				*(tektontest.MakePRCompletion(clock, "otherSecond", ns, success, labels, 10)),
			},
			wantName: []string{"otherFirst", "otherSecond", "notStarted"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			PipelineRunSortByStartTime(tt.pruns)
			for key, value := range tt.pruns {
				assert.Equal(t, tt.wantName[key], value.GetName())
			}
		})
	}
}
