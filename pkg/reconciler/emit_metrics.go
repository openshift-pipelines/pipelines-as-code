package reconciler

import (
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

func (r *Reconciler) emitMetrics(pr *tektonv1.PipelineRun) error {
	gitProvider := pr.GetLabels()[keys.GitProvider]
	eventType := pr.GetLabels()[keys.EventType]

	if strings.HasPrefix(gitProvider, "github") {
		if _, ok := pr.GetAnnotations()[keys.InstallationID]; ok {
			gitProvider += "-app"
		} else {
			gitProvider += "-webhook"
		}
	}

	return r.metrics.Count(gitProvider, eventType)
}
