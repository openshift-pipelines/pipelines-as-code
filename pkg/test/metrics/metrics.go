package metrics

import (
	"github.com/openshift-pipelines/pipelines-as-code/pkg/metrics"
	"knative.dev/pkg/metrics/metricstest"
)

func ResetMetrics() {
	metricstest.Unregister(
		"pipelines_as_code_pipelinerun_count",
		"pipelines_as_code_pipelinerun_duration_seconds_sum",
		"pipelines_as_code_running_pipelineruns_count",
		"pipelines_as_code_git_provider_api_request_count",
	)

	// have to reset sync.Once to allow recreation of Recorder.
	metrics.ResetRecorder()
}
