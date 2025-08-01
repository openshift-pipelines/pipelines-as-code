package sort

import (
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	tektontest "github.com/openshift-pipelines/pipelines-as-code/pkg/test/tekton"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPipelineRunSortByCompletionTime(t *testing.T) {
	clock := clockwork.NewFakeClock()
	ns := "namespace"
	labels := map[string]string{}
	success := tektonv1.PipelineRunReasonSuccessful.String()

	pruns := []tektonv1.PipelineRun{
		*(tektontest.MakePRCompletion(clock, "troisieme", ns, success, nil, labels, 30)),
		*(tektontest.MakePRCompletion(clock, "premier", ns, success, nil, labels, 10)),
		*(tektontest.MakePRCompletion(clock, "second", ns, success, nil, labels, 20)),
	}

	prunsMissing := []tektonv1.PipelineRun{
		*(tektontest.MakePRCompletion(clock, "troisieme", ns, success, nil, labels, 30)),
		*(tektontest.MakePRCompletion(clock, "premier", ns, success, nil, labels, 10)),
		*(tektontest.MakePRCompletion(clock, "no-completion-time", ns, success, nil, labels, 20)),
	}
	for i := range prunsMissing {
		if prunsMissing[i].Name == "no-completion-time" {
			prunsMissing[i].Status.CompletionTime = nil
		}
	}

	prunsWithOneMissing := []tektonv1.PipelineRun{
		*(tektontest.MakePRCompletion(clock, "premier", ns, success, nil, labels, 10)),
		*(tektontest.MakePRCompletion(clock, "no-completion-time", ns, success, nil, labels, 20)),
	}
	for i := range prunsWithOneMissing {
		if prunsWithOneMissing[i].Name == "no-completion-time" {
			prunsWithOneMissing[i].Status.CompletionTime = nil
		}
	}

	prunsWithJMissing := []tektonv1.PipelineRun{
		*(tektontest.MakePRCompletion(clock, "no-completion-time", ns, success, nil, labels, 20)),
		*(tektontest.MakePRCompletion(clock, "premier", ns, success, nil, labels, 10)),
	}
	for i := range prunsWithJMissing {
		if prunsWithJMissing[i].Name == "no-completion-time" {
			prunsWithJMissing[i].Status.CompletionTime = nil
		}
	}

	tests := []struct {
		name     string
		pruns    []tektonv1.PipelineRun
		wantName []string
	}{
		{
			name:     "sort by completion time",
			pruns:    pruns,
			wantName: []string{"premier", "second", "troisieme"},
		},
		{
			name:     "sort by completion time with missing",
			pruns:    prunsMissing,
			wantName: []string{"no-completion-time", "premier", "troisieme"},
		},
		{
			name:     "sort by completion time with one missing",
			pruns:    prunsWithOneMissing,
			wantName: []string{"no-completion-time", "premier"},
		},
		{
			name:     "sort by completion time with j missing",
			pruns:    prunsWithJMissing,
			wantName: []string{"no-completion-time", "premier"},
		},
		{
			name:     "empty list",
			pruns:    []tektonv1.PipelineRun{},
			wantName: []string{},
		},
		{
			name:     "single item",
			pruns:    []tektonv1.PipelineRun{*(tektontest.MakePRCompletion(clock, "premier", ns, success, nil, labels, 10))},
			wantName: []string{"premier"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PipelineRunSortByCompletionTime(tt.pruns)
			for key, value := range got {
				assert.Equal(t, tt.wantName[key], value.GetName())
			}
		})
	}
}

func TestPipelineRunSortByStartTime(t *testing.T) {
	clock := clockwork.NewFakeClock()
	ns := "namespace"
	labels := map[string]string{}
	success := tektonv1.PipelineRunReasonSuccessful.String()
	startedEarlierPR := tektontest.MakePRCompletion(clock, "earlier", ns, success, nil, labels, 5)
	startedEarlierPR.Status.StartTime = &metav1.Time{Time: clock.Now().Add(100 * time.Minute)}

	noCompletionPR := tektontest.MakePRCompletion(clock, "noCompletion", ns, success, nil, labels, 5)
	noCompletionPR.Status.StartTime = &metav1.Time{Time: clock.Now().Add(500 * time.Minute)}
	noCompletionPR.Status.CompletionTime = nil

	notStartedYet := tektontest.MakePRCompletion(clock, "notStarted", ns, success, nil, labels, 5)
	notStartedYet.Status.StartTime = nil
	notStartedYet.Status.CompletionTime = nil

	prunsWithOneNotStarted := []tektonv1.PipelineRun{
		*(tektontest.MakePRCompletion(clock, "otherFirst", ns, success, nil, labels, 30)),
		*notStartedYet,
	}

	prunsWithJNotStarted := []tektonv1.PipelineRun{
		*notStartedYet,
		*(tektontest.MakePRCompletion(clock, "otherFirst", ns, success, nil, labels, 30)),
	}

	tests := []struct {
		name     string
		pruns    []tektonv1.PipelineRun
		wantName []string
	}{
		{
			name: "finished last started first",
			pruns: []tektonv1.PipelineRun{
				*(tektontest.MakePRCompletion(clock, "otherFirst", ns, success, nil, labels, 30)),
				*(tektontest.MakePRCompletion(clock, "otherSecond", ns, success, nil, labels, 10)),
				*startedEarlierPR,
			},
			wantName: []string{"earlier", "otherFirst", "otherSecond"},
		},
		{
			name: "no completion but started first",
			pruns: []tektonv1.PipelineRun{
				*noCompletionPR,
				*(tektontest.MakePRCompletion(clock, "otherFirst", ns, success, nil, labels, 30)),
				*(tektontest.MakePRCompletion(clock, "otherSecond", ns, success, nil, labels, 10)),
			},
			wantName: []string{"noCompletion", "otherFirst", "otherSecond"},
		},

		{
			name: "not started yet",
			pruns: []tektonv1.PipelineRun{
				*(tektontest.MakePRCompletion(clock, "otherFirst", ns, success, nil, labels, 30)),
				*(tektontest.MakePRCompletion(clock, "otherSecond", ns, success, nil, labels, 10)),
				*notStartedYet,
			},
			wantName: []string{"notStarted", "otherFirst", "otherSecond"},
		},
		{
			name:     "not started yet single",
			pruns:    prunsWithOneNotStarted,
			wantName: []string{"notStarted", "otherFirst"},
		},
		{
			name:     "not started yet single j",
			pruns:    prunsWithJNotStarted,
			wantName: []string{"notStarted", "otherFirst"},
		},
		{
			name:     "empty list",
			pruns:    []tektonv1.PipelineRun{},
			wantName: []string{},
		},
		{
			name:     "single item",
			pruns:    []tektonv1.PipelineRun{*startedEarlierPR},
			wantName: []string{"earlier"},
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
