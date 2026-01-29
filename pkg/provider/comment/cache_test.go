package comment

import (
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/test/logger"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestGetCachedCommentID(t *testing.T) {
	ctx, _ := rtesting.SetupFakeContext(t)
	testLogger, _ := logger.GetLogger()

	tests := []struct {
		name         string
		pr           *tektonv1.PipelineRun
		pipelineName string
		want         int64
	}{
		{
			name: "cached comment ID exists in current PipelineRun",
			pr: &tektonv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-run",
					Namespace: "default",
					Annotations: map[string]string{
						keys.StatusCommentID + "-my-pipeline": "12345",
					},
				},
			},
			pipelineName: "my-pipeline",
			want:         12345,
		},
		{
			name: "no annotation exists",
			pr: &tektonv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-run",
					Namespace:   "default",
					Annotations: map[string]string{},
				},
			},
			pipelineName: "my-pipeline",
			want:         0,
		},
		{
			name:         "nil PipelineRun",
			pr:           nil,
			pipelineName: "my-pipeline",
			want:         0,
		},
		{
			name: "invalid comment ID format",
			pr: &tektonv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-run",
					Namespace: "default",
					Annotations: map[string]string{
						keys.StatusCommentID + "-my-pipeline": "not-a-number",
					},
				},
			},
			pipelineName: "my-pipeline",
			want:         0,
		},
		{
			name: "different pipeline name",
			pr: &tektonv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-run",
					Namespace: "default",
					Annotations: map[string]string{
						keys.StatusCommentID + "-other-pipeline": "99999",
					},
				},
			},
			pipelineName: "my-pipeline",
			want:         0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namespace := ""
			if tt.pr != nil {
				namespace = tt.pr.GetNamespace()
			}
			got := GetCachedCommentID(ctx, testLogger, nil, tt.pr, namespace, tt.pipelineName, 1)
			assert.Equal(t, got, tt.want)
		})
	}
}

func TestCacheCommentID(t *testing.T) {
	testLogger, _ := logger.GetLogger()

	tests := []struct {
		name         string
		prName       string
		pipelineName string
		commentID    int64
		wantErr      bool
	}{
		{
			name:         "cache comment ID successfully",
			prName:       "test-run-1",
			pipelineName: "my-pipeline",
			commentID:    54321,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			pr := &tektonv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:        tt.prName,
					Namespace:   "default",
					Annotations: map[string]string{},
				},
			}

			stdata, _ := testclient.SeedTestData(t, ctx, testclient.Data{
				PipelineRuns: []*tektonv1.PipelineRun{pr},
			})

			err := CacheCommentID(ctx, testLogger, stdata.Pipeline, pr, tt.pipelineName, tt.commentID)
			if tt.wantErr {
				assert.Assert(t, err != nil)
				return
			}
			assert.NilError(t, err)
		})
	}
}

func TestCacheCommentIDWithNilClient(t *testing.T) {
	ctx, _ := rtesting.SetupFakeContext(t)
	testLogger, _ := logger.GetLogger()

	pr := &tektonv1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-run",
			Namespace: "default",
		},
	}

	err := CacheCommentID(ctx, testLogger, nil, pr, "my-pipeline", 12345)
	assert.Assert(t, err != nil, "Should return error when tektonClient is nil")
}
