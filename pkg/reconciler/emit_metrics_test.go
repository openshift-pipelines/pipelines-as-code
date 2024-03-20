package reconciler

import (
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/metrics"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestEmitMetrics(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		wantErr     bool
	}{
		{
			name: "provider is GitHub App",
			annotations: map[string]string{
				keys.GitProvider:    "github",
				keys.EventType:      "pull_request",
				keys.InstallationID: "123",
			},
			wantErr: false,
		},
		{
			name: "provider is GitHub Enterprise App",
			annotations: map[string]string{
				keys.GitProvider:    "github-enterprise",
				keys.EventType:      "pull_request",
				keys.InstallationID: "123",
			},
			wantErr: false,
		},
		{
			name: "provider is GitHub Webhook",
			annotations: map[string]string{
				keys.GitProvider: "github",
				keys.EventType:   "pull_request",
			},
			wantErr: false,
		},
		{
			name: "provider is GitLab",
			annotations: map[string]string{
				keys.GitProvider: "gitlab",
				keys.EventType:   "push",
			},
			wantErr: false,
		},
		{
			name: "unsupported provider",
			annotations: map[string]string{
				keys.GitProvider: "unsupported",
				keys.EventType:   "push",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := metrics.NewRecorder()
			assert.NilError(t, err)
			r := &Reconciler{
				metrics: m,
			}
			pr := &tektonv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: tt.annotations,
				},
			}
			if err = r.emitMetrics(pr); (err != nil) != tt.wantErr {
				t.Errorf("emitMetrics() error = %v, wantErr %v", err != nil, tt.wantErr)
			}
		})
	}
}
