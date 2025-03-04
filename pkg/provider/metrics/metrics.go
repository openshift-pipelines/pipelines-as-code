package metrics

import (
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/metrics"
	"go.uber.org/zap"
)

func RecordAPIUsage(logger *zap.SugaredLogger, provider, eventType string, repo *v1alpha1.Repository) {
	recorder, err := metrics.NewRecorder()
	if err != nil {
		logger.Errorf("Error initializing metrics recorder: %v", err)
	}
	repoName := ""
	namespace := ""
	if repo != nil {
		repoName = repo.Name
		namespace = repo.Namespace
	}

	if err := recorder.ReportGitProviderAPIUsage(provider, eventType, namespace, repoName); err != nil {
		logger.Errorf("Error reporting git API usage metrics for %q repository %q in %q namespace: %v", provider, namespace, repoName, err)
	}
}
