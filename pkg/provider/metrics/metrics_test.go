package metrics

import (
	"testing"

	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/metrics/metricstest"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	metricsutils "github.com/openshift-pipelines/pipelines-as-code/pkg/test/metrics"

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
			defer metricsutils.ResetMetrics()

			observer, _ := zapobserver.New(zap.InfoLevel)
			fakelogger := zap.New(observer).Sugar()

			RecordAPIUsage(fakelogger, testCase.provider, testCase.eventType, testCase.repo)

			metricstest.CheckCountData(t, "pipelines_as_code_git_provider_api_request_count", testCase.wantTags, 1)
		})
	}
}
