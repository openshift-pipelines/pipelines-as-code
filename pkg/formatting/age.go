package formatting

import (
	"fmt"
	"time"

	"github.com/hako/durafmt"
	"github.com/jonboulle/clockwork"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Age(t *metav1.Time, c clockwork.Clock) string {
	if t.IsZero() {
		return nonAttributedStr
	}

	dur := c.Since(t.Time)
	return durafmt.ParseShort(dur).String() + " ago"
}

func Duration(t1, t2 *metav1.Time) string {
	if t1.IsZero() || t2.IsZero() {
		return nonAttributedStr
	}

	dur := t2.Time.Sub(t1.Time)
	return durafmt.ParseShort(dur).String()
}

func PRDuration(pr v1alpha1.RepositoryRunStatus) string {
	if pr.StartTime == nil {
		return nonAttributedStr
	}

	lasttime := pr.CompletionTime
	if lasttime == nil {
		lasttime = &pr.Status.Conditions[0].LastTransitionTime.Inner
	}

	return Duration(pr.StartTime, lasttime)
}

func HumanDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	m := d / time.Minute
	return fmt.Sprintf("%02d", m)
}

func Timeout(t *metav1.Duration) string {
	if t == nil {
		return nonAttributedStr
	}

	return durafmt.Parse(t.Duration).String()
}
