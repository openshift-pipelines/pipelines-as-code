package reconciler

import (
	"testing"
	"time"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/metrics"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/metrics/metricstest"
	_ "knative.dev/pkg/metrics/testing"
)

// TestCountPipelineRun tests pipelinerun count metric.
func TestCountPipelineRun(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		tags        map[string]string
		wantErr     bool
	}{
		{
			name: "provider is GitHub App",
			annotations: map[string]string{
				keys.GitProvider:    "github",
				keys.EventType:      "pull_request",
				keys.InstallationID: "123",
			},
			tags: map[string]string{
				"provider":   "github-app",
				"event-type": "pull_request",
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
			tags: map[string]string{
				"provider":   "github-enterprise-app",
				"event-type": "pull_request",
			},
			wantErr: false,
		},
		{
			name: "provider is GitHub Webhook",
			annotations: map[string]string{
				keys.GitProvider: "github",
				keys.EventType:   "pull_request",
			},
			tags: map[string]string{
				"provider":   "github-webhook",
				"event-type": "pull_request",
			},
			wantErr: false,
		},
		{
			name: "provider is GitLab",
			annotations: map[string]string{
				keys.GitProvider: "gitlab",
				keys.EventType:   "push",
			},
			tags: map[string]string{
				"provider":   "gitlab-webhook",
				"event-type": "push",
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
			unregisterMetrics()
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
			// checks that metric is unregistered successfully and there is no metric
			// before emitting new pr count metric.
			metricstest.AssertNoMetric(t, "pipelines_as_code_pipelinerun_count")

			if err = r.countPipelineRun(pr); (err != nil) != tt.wantErr {
				t.Errorf("countPipelineRun() error = %v, wantErr %v", err != nil, tt.wantErr)
			}

			if !tt.wantErr {
				metricstest.CheckCountData(t, "pipelines_as_code_pipelinerun_count", tt.tags, 1)
			}
		})
	}
}

// TestCalculatePipelineRunDuration tests pipelinerun duration metric.
func TestCalculatePipelineRunDuration(t *testing.T) {
	startTime := metav1.Now()
	tests := []struct {
		name           string
		annotations    map[string]string
		conditionType  apis.ConditionType
		status         corev1.ConditionStatus
		reason         string
		completionTime metav1.Time
		tags           map[string]string
	}{
		{
			name: "pipelinerun succeeded",
			annotations: map[string]string{
				keys.Repository: "pac-repo",
			},
			conditionType:  apis.ConditionSucceeded,
			status:         corev1.ConditionTrue,
			reason:         tektonv1.PipelineRunReasonSuccessful.String(),
			completionTime: metav1.NewTime(startTime.Time.Add(time.Minute)),
			tags: map[string]string{
				"namespace":  "pac-ns",
				"reason":     tektonv1.PipelineRunReasonSuccessful.String(),
				"repository": "pac-repo",
				"status":     "success",
			},
		},
		{
			name: "pipelinerun completed",
			annotations: map[string]string{
				keys.Repository: "pac-repo",
			},
			conditionType:  apis.ConditionSucceeded,
			status:         corev1.ConditionTrue,
			reason:         tektonv1.PipelineRunReasonCompleted.String(),
			completionTime: metav1.NewTime(startTime.Time.Add(time.Minute)),
			tags: map[string]string{
				"namespace":  "pac-ns",
				"reason":     tektonv1.PipelineRunReasonCompleted.String(),
				"repository": "pac-repo",
				"status":     "success",
			},
		},
		{
			name: "pipelinerun failed",
			annotations: map[string]string{
				keys.Repository: "pac-repo",
			},
			conditionType:  apis.ConditionSucceeded,
			status:         corev1.ConditionFalse,
			reason:         tektonv1.PipelineRunReasonFailed.String(),
			completionTime: metav1.NewTime(startTime.Time.Add(2 * time.Minute)),
			tags: map[string]string{
				"namespace":  "pac-ns",
				"reason":     tektonv1.PipelineRunReasonFailed.String(),
				"repository": "pac-repo",
				"status":     "failed",
			},
		},
		{
			name: "pipelinerun cancelled",
			annotations: map[string]string{
				keys.Repository: "pac-repo",
			},
			conditionType:  apis.ConditionSucceeded,
			status:         corev1.ConditionFalse,
			reason:         tektonv1.PipelineRunReasonCancelled.String(),
			completionTime: metav1.NewTime(startTime.Time.Add(2 * time.Second)),
			tags: map[string]string{
				"namespace":  "pac-ns",
				"reason":     tektonv1.PipelineRunReasonCancelled.String(),
				"repository": "pac-repo",
				"status":     "cancelled",
			},
		},
		{
			name: "pipelinerun timed out",
			annotations: map[string]string{
				keys.Repository: "pac-repo",
			},
			conditionType:  apis.ConditionSucceeded,
			status:         corev1.ConditionFalse,
			reason:         tektonv1.PipelineRunReasonTimedOut.String(),
			completionTime: metav1.NewTime(startTime.Time.Add(10 * time.Minute)),
			tags: map[string]string{
				"namespace":  "pac-ns",
				"reason":     tektonv1.PipelineRunReasonTimedOut.String(),
				"repository": "pac-repo",
				"status":     "failed",
			},
		},
		{
			name: "pipelinerun failed due to couldn't get pipeline",
			annotations: map[string]string{
				keys.Repository: "pac-repo",
			},
			conditionType:  apis.ConditionSucceeded,
			status:         corev1.ConditionFalse,
			reason:         tektonv1.PipelineRunReasonCouldntGetPipeline.String(),
			completionTime: metav1.NewTime(startTime.Time.Add(time.Second)),
			tags: map[string]string{
				"namespace":  "pac-ns",
				"reason":     tektonv1.PipelineRunReasonCouldntGetPipeline.String(),
				"repository": "pac-repo",
				"status":     "failed",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unregisterMetrics()
			m, err := metrics.NewRecorder()
			assert.NilError(t, err)
			r := &Reconciler{
				metrics: m,
			}
			pr := &tektonv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:   "pac-ns",
					Annotations: tt.annotations,
				},
				Status: tektonv1.PipelineRunStatus{
					Status: duckv1.Status{Conditions: []apis.Condition{
						{
							Type:   tt.conditionType,
							Status: tt.status,
							Reason: tt.reason,
						},
					}},
					PipelineRunStatusFields: tektonv1.PipelineRunStatusFields{
						StartTime:      &startTime,
						CompletionTime: &tt.completionTime,
					},
				},
			}
			// checks that metric is unregistered successfully and there is no metric
			// before emitting new pr duration metric.
			metricstest.AssertNoMetric(t, "pipelines_as_code_pipelinerun_duration_seconds_sum")

			if err = r.calculatePRDuration(pr); err != nil {
				t.Errorf("calculatePRDuration() error = %v", err)
			}

			duration := tt.completionTime.Sub(startTime.Time)
			metricstest.CheckSumData(t, "pipelines_as_code_pipelinerun_duration_seconds_sum", tt.tags, duration.Seconds())
		})
	}
}

func TestCountRunningPRs(t *testing.T) {
	annotations := map[string]string{
		keys.GitProvider: "github",
		keys.EventType:   "pull_request",
		keys.Repository:  "pac-repo",
	}
	var prl []*tektonv1.PipelineRun
	pr := &tektonv1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   "pac-ns",
			Annotations: annotations,
		},
		Status: tektonv1.PipelineRunStatus{
			Status: duckv1.Status{Conditions: []apis.Condition{
				{
					Type:   apis.ConditionReady,
					Status: corev1.ConditionTrue,
					Reason: tektonv1.PipelineRunReasonRunning.String(),
				},
			}},
		},
	}

	numberOfRunningPRs := 10
	for i := 0; i < numberOfRunningPRs; i++ {
		prl = append(prl, pr)
	}

	unregisterMetrics()
	m, err := metrics.NewRecorder()
	assert.NilError(t, err)
	r := &Reconciler{
		metrics: m,
	}

	err = r.metrics.EmitRunningPRsMetrics(prl)
	assert.NilError(t, err)
	tags := map[string]string{
		"namespace":  "pac-ns",
		"repository": "pac-repo",
	}
	metricstest.CheckLastValueData(t, "pipelines_as_code_running_pipelineruns_count", tags, float64(numberOfRunningPRs))
}

func unregisterMetrics() {
	metricstest.Unregister("pipelines_as_code_pipelinerun_count",
		"pipelines_as_code_pipelinerun_duration_seconds_sum",
		"pipelines_as_code_running_pipelineruns_count")

	// have to reset sync.Once to allow recreation of Recorder.
	metrics.ResetRecorder()
}
