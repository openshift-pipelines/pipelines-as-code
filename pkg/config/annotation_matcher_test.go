package config

import (
	"reflect"
	"strings"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMatchPipelinerunByAnnotation(t *testing.T) {
	pipelineGood := &tektonv1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipeline-good",
			Annotations: map[string]string{
				pipelinesascode.GroupName + "/" + onEventAnnotation:        "[pull_request]",
				pipelinesascode.GroupName + "/" + onTargetBranchAnnotation: "[main]",
			},
		},
	}

	pipelineOther := &tektonv1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipeline-other",
			Annotations: map[string]string{
				pipelinesascode.GroupName + "/" + onEventAnnotation:        "[pull_request]",
				pipelinesascode.GroupName + "/" + onTargetBranchAnnotation: "[main]",
			},
		},
	}

	observer, log := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()
	cs := &cli.Clients{
		Log: logger,
	}

	type args struct {
		pruns   []*tektonv1beta1.PipelineRun
		runinfo *webvcs.RunInfo
	}
	tests := []struct {
		name       string
		args       args
		wantErr    bool
		wantPrName string
		wantLog    string
	}{
		{
			name: "good-match-with-only-one",
			args: args{
				pruns:   []*tektonv1beta1.PipelineRun{pipelineGood},
				runinfo: &webvcs.RunInfo{EventType: "pull_request", BaseBranch: "main"},
			},
			wantErr:    false,
			wantPrName: "pipeline-good",
		},
		{
			name: "first-one-match-with-two-good-ones",
			args: args{
				pruns:   []*tektonv1beta1.PipelineRun{pipelineGood, pipelineOther},
				runinfo: &webvcs.RunInfo{EventType: "pull_request", BaseBranch: "main"},
			},
			wantErr:    false,
			wantPrName: "pipeline-good",
		},
		{
			name: "no-match-on-event",
			args: args{
				pruns:   []*tektonv1beta1.PipelineRun{pipelineGood, pipelineOther},
				runinfo: &webvcs.RunInfo{EventType: "push", BaseBranch: "main"},
			},
			wantErr: true,
		},
		{
			name: "no-match-on-target-branch",
			args: args{
				pruns:   []*tektonv1beta1.PipelineRun{pipelineGood, pipelineOther},
				runinfo: &webvcs.RunInfo{EventType: "pull_request", BaseBranch: "other"},
			},
			wantErr: true,
		},
		{
			name: "no-annotation",
			args: args{
				pruns: []*tektonv1beta1.PipelineRun{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pipeline-other",
						},
					},
				},
				runinfo: &webvcs.RunInfo{EventType: "push", BaseBranch: "main"},
			},
			wantErr: true,
			wantLog: "does not have any annotations",
		},
		{
			name: "bad-event-annotation",
			args: args{
				runinfo: &webvcs.RunInfo{EventType: "pull_request", BaseBranch: "main"},
				pruns: []*tektonv1beta1.PipelineRun{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "bad-event-annotation",
							Annotations: map[string]string{
								pipelinesascode.GroupName + "/" + onEventAnnotation:        "pull_request",
								pipelinesascode.GroupName + "/" + onTargetBranchAnnotation: "[main]",
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "bad-target-branch-annotation",
			args: args{
				runinfo: &webvcs.RunInfo{EventType: "pull_request", BaseBranch: "main"},
				pruns: []*tektonv1beta1.PipelineRun{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "bad-target-branch-annotation",
							Annotations: map[string]string{
								pipelinesascode.GroupName + "/" + onEventAnnotation:        "[pull_request]",
								pipelinesascode.GroupName + "/" + onTargetBranchAnnotation: "main",
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "empty-annotation",
			args: args{
				runinfo: &webvcs.RunInfo{EventType: "pull_request", BaseBranch: "main"},
				pruns: []*tektonv1beta1.PipelineRun{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "bad-target-branch-annotation",
							Annotations: map[string]string{
								pipelinesascode.GroupName + "/" + onEventAnnotation:        "[]",
								pipelinesascode.GroupName + "/" + onTargetBranchAnnotation: "[]",
							},
						},
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MatchPipelinerunByAnnotation(tt.args.pruns, cs, tt.args.runinfo)
			if (err != nil) != tt.wantErr {
				t.Errorf("MatchPipelinerunByAnnotation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantPrName != "" {
				assert.Assert(t, got.GetName() == tt.wantPrName, "Pipelinerun hasn't been matched: "+got.GetName()+"!="+tt.wantPrName)
			}
			if tt.wantLog != "" {
				logmsg := log.TakeAll()
				assert.Assert(t, len(logmsg) > 0)
				assert.Assert(t, strings.Contains(logmsg[0].Message, tt.wantLog))
			}
		})
	}
}

func Test_getAnnotationValues(t *testing.T) {
	type args struct {
		annotation string
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "get-annotation-simple",
			args: args{
				annotation: "[foo]",
			},
			want:    []string{"foo"},
			wantErr: false,
		},
		{
			name: "get-annotation-multiples",
			args: args{
				annotation: "[foo, bar]",
			},
			want:    []string{"foo", "bar"},
			wantErr: false,
		},
		{
			name: "get-annotation-bad-syntax",
			args: args{
				annotation: "foo]",
			},
			wantErr: true,
		},
		{
			name: "get-annotation-error-empty",
			args: args{
				annotation: "[]",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getAnnotationValues(tt.args.annotation)
			if (err != nil) != tt.wantErr {
				t.Errorf("getAnnotationValues() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getAnnotationValues() got = %v, want %v", got, tt.want)
			}
		})
	}
}
