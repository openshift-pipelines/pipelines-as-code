package formatting

import (
	"github.com/hako/durafmt"
	"github.com/jonboulle/clockwork"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Age(t *metav1.Time, c clockwork.Clock) string {
	if t.IsZero() {
		return nonAttributedStr
	}
	return durafmt.ParseShort(c.Since(t.Time)).String() + " ago"
}

func Duration(t1, t2 *metav1.Time) string {
	if t1.IsZero() || t2.IsZero() {
		return nonAttributedStr
	}
	return durafmt.ParseShort(t2.Sub(t1.Time)).String()
}

// PRDuration calculates the duration of a repository run, given its status.
// It takes a RepositoryRunStatus object as input.
// It returns a string with the duration of the run, or nonAttributedStr if the run has not started or completed.
func PRDuration(runStatus v1alpha1.RepositoryRunStatus) string {
	if runStatus.StartTime == nil {
		return nonAttributedStr
	}

	lasttime := runStatus.CompletionTime
	if lasttime == nil {
		if len(runStatus.Conditions) == 0 {
			return nonAttributedStr
		}
		lasttime = &runStatus.Conditions[0].LastTransitionTime.Inner
	}

	return Duration(runStatus.StartTime, lasttime)
}

func Timeout(t *metav1.Duration) string {
	if t == nil {
		return nonAttributedStr
	}
	return durafmt.Parse(t.Duration).String()
}
