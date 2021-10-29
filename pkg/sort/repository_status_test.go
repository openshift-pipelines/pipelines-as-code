package sort

import (
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func makeRepositoryRunStatus(clock clockwork.FakeClock, prName string, timeshift int) v1alpha1.RepositoryRunStatus {
	starttime := time.Duration((timeshift - 5*-1) * int(time.Minute))
	endtime := time.Duration((timeshift * -1) * int(time.Minute))

	return v1alpha1.RepositoryRunStatus{
		PipelineRunName: prName,
		StartTime:       &metav1.Time{Time: clock.Now().Add(starttime)},
		CompletionTime:  &metav1.Time{Time: clock.Now().Add(endtime)},
	}
}

func TestRepositoryRunStatus(t *testing.T) {
	cw := clockwork.NewFakeClock()
	tests := []struct {
		name   string
		wantPR []string
		repos  []v1alpha1.RepositoryRunStatus
	}{
		{
			name: "sortit",
			repos: []v1alpha1.RepositoryRunStatus{
				makeRepositoryRunStatus(cw, "second", 20),
				makeRepositoryRunStatus(cw, "first", 10),
				makeRepositoryRunStatus(cw, "third", 30),
			},
			// we want reverse sort for tkn pac at least
			wantPR: []string{"third", "second", "first"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Run(tt.name, func(t *testing.T) {
				for key, value := range RepositorySortRunStatus(tt.repos) {
					assert.Equal(t, tt.wantPR[key], value.PipelineRunName)
				}
			})
		})
	}
}
