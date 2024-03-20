package reconciler

import (
	"fmt"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

func (r *Reconciler) emitMetrics(pr *tektonv1.PipelineRun) error {
	gitProvider := pr.GetAnnotations()[keys.GitProvider]
	eventType := pr.GetAnnotations()[keys.EventType]

	switch gitProvider {
	case "github", "github-enterprise":
		if _, ok := pr.GetAnnotations()[keys.InstallationID]; ok {
			gitProvider += "-app"
		} else {
			gitProvider += "-webhook"
		}
	case "gitlab", "gitea", "bitbucket-cloud", "bitbucket-server":
		gitProvider += "-webhook"
	default:
		return fmt.Errorf("no supported Git provider")
	}

	return r.metrics.Count(gitProvider, eventType)
}
