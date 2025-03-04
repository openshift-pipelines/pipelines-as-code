package metrics

import (
	"testing"

	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/metrics/metricstest"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/metrics"

	_ "knative.dev/pkg/metrics/testing"
)

func TestRecordAPIUsage(t *testing.T) {
	type testCaseData struct {
		desc      string
		provider  string
		eventType string
		repo      *v1alpha1.Repository
		wantTags  map[string]string
	}

	testCases := []testCaseData{
		{
			desc:      "when repository is not nil",
			provider:  "github",
			eventType: "pull_request",
			repo: &v1alpha1.Repository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-name",
					Namespace: "test-namespace",
				},
			},
			wantTags: map[string]string{"provider": "github", "event-type": "pull_request", "namespace": "test-namespace", "repository": "test-name"},
		},
		{
			desc:      "when repository is nil",
			provider:  "github",
			eventType: "pull_request",
			repo:      nil,
			wantTags:  map[string]string{"provider": "github", "event-type": "pull_request"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.desc, func(t *testing.T) {
			defer func() {
				metricstest.Unregister(
					"pipelines_as_code_pipelinerun_count",
					"pipelines_as_code_pipelinerun_duration_seconds_sum",
					"pipelines_as_code_running_pipelineruns_count",
					"pipelines_as_code_git_provider_api_request_count",
				)
				metrics.ResetRecorder()
			}()

			observer, _ := zapobserver.New(zap.InfoLevel)
			fakelogger := zap.New(observer).Sugar()

			RecordAPIUsage(fakelogger, testCase.provider, testCase.eventType, testCase.repo)

			metricstest.CheckCountData(t, "pipelines_as_code_git_provider_api_request_count", testCase.wantTags, 1)
		})
	}
}
