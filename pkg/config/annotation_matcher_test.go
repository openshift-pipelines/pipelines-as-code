package config

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	testnewrepo "github.com/openshift-pipelines/pipelines-as-code/pkg/test/repository"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestMatchPipelinerunAnnotationAndRepositories(t *testing.T) {
	pipelineTargetNSName := "pipeline-target-ns"
	pipelineTargetNS := &tektonv1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: pipelineTargetNSName,
			Annotations: map[string]string{
				pipelinesascode.GroupName + "/" + onTargetNamespace:        targetNamespace,
				pipelinesascode.GroupName + "/" + onEventAnnotation:        "[pull_request]",
				pipelinesascode.GroupName + "/" + onTargetBranchAnnotation: fmt.Sprintf("[%s]", mainBranch),
				pipelinesascode.GroupName + "/" + maxKeepRuns:              "2",
			},
		},
	}

	type args struct {
		pruns   []*tektonv1beta1.PipelineRun
		runinfo *webvcs.RunInfo
		data    testclient.Data
	}
	tests := []struct {
		name, wantPRName, wantRepoName, wantLog string
		args                                    args
		wantErr                                 bool
	}{
		{
			name:       "match a repository with target NS",
			wantPRName: pipelineTargetNSName,
			args: args{
				pruns:   []*tektonv1beta1.PipelineRun{pipelineTargetNS},
				runinfo: &webvcs.RunInfo{URL: targetURL, EventType: "pull_request", BaseBranch: mainBranch},
				data: testclient.Data{
					Repositories: []*v1alpha1.Repository{
						testnewrepo.NewRepo("test-good", targetURL, mainBranch, targetNamespace, targetNamespace, "pull_request"),
					},
				},
			},
		},
		{
			name:         "match same webhook on multiple repos with different NS",
			wantPRName:   pipelineTargetNSName,
			wantRepoName: "test-good",
			args: args{
				pruns:   []*tektonv1beta1.PipelineRun{pipelineTargetNS},
				runinfo: &webvcs.RunInfo{URL: targetURL, EventType: "pull_request", BaseBranch: mainBranch},
				data: testclient.Data{
					Repositories: []*v1alpha1.Repository{
						testnewrepo.NewRepo("test-other", targetURL, mainBranch, targetNamespace, targetNamespace, "pull_request"),
						testnewrepo.NewRepo("test-good", targetURL, mainBranch, targetNamespace, targetNamespace, "pull_request"),
					},
				},
			},
		},
		{
			name:    "no match a repository with target NS",
			wantErr: true,
			wantLog: "could not find Repository CRD",
			args: args{
				pruns:   []*tektonv1beta1.PipelineRun{pipelineTargetNS},
				runinfo: &webvcs.RunInfo{URL: targetURL, EventType: "pull_request", BaseBranch: mainBranch},
				data: testclient.Data{
					Repositories: []*v1alpha1.Repository{
						testnewrepo.NewRepo("test-good", targetURL, mainBranch, "otherNS", "otherNS", "pull_request"),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			cs, _ := testclient.SeedTestData(t, ctx, tt.args.data)
			observer, log := zapobserver.New(zap.InfoLevel)
			logger := zap.New(observer).Sugar()
			client := &cli.Clients{PipelineAsCode: cs.PipelineAsCode, Log: logger}
			got, repo, _, err := MatchPipelinerunByAnnotation(ctx,
				tt.args.pruns,
				client, tt.args.runinfo)

			if tt.wantErr && err == nil {
				t.Error("We should have get an error")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("We should have not get an error %s", err)
			}

			if tt.wantRepoName != "" {
				assert.Assert(t, tt.wantRepoName == repo.GetName())
			}
			if tt.wantPRName != "" {
				assert.Assert(t, tt.wantPRName == got.GetName())
			}
			if tt.wantLog != "" {
				logmsg := log.TakeAll()
				assert.Assert(t, len(logmsg) > 0, "We didn't get any log message")
				assert.Assert(t, strings.Contains(logmsg[0].Message, tt.wantLog), logmsg[0].Message, tt.wantLog)
			}
		})
	}
}

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
			wantLog: "cannot match between event and pipelineRuns",
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
		{
			name: "match-branch-matching",
			args: args{
				runinfo: &webvcs.RunInfo{EventType: "push", BaseBranch: "refs/heads/main"},
				pruns: []*tektonv1beta1.PipelineRun{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "branch-matching",
							Annotations: map[string]string{
								pipelinesascode.GroupName + "/" + onEventAnnotation:        "[push]",
								pipelinesascode.GroupName + "/" + onTargetBranchAnnotation: "[main]",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "base-does-not-compare",
			args: args{
				runinfo: &webvcs.RunInfo{EventType: "push", BaseBranch: "refs/heads/main/foobar"},
				pruns: []*tektonv1beta1.PipelineRun{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "branch-base-matching-not-compare",
							Annotations: map[string]string{
								pipelinesascode.GroupName + "/" + onEventAnnotation:        "[push]",
								pipelinesascode.GroupName + "/" + onTargetBranchAnnotation: "[main]",
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "branch-glob-matching",
			args: args{
				runinfo: &webvcs.RunInfo{EventType: "push", BaseBranch: "refs/heads/main"},
				pruns: []*tektonv1beta1.PipelineRun{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "branch-base-matching-not-compare",
							Annotations: map[string]string{
								pipelinesascode.GroupName + "/" + onEventAnnotation:        "[push]",
								pipelinesascode.GroupName + "/" + onTargetBranchAnnotation: "[refs/heads/*]",
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			got, _, _, err := MatchPipelinerunByAnnotation(ctx, tt.args.pruns, cs, tt.args.runinfo)
			if (err != nil) != tt.wantErr {
				t.Errorf("MatchPipelinerunByAnnotation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantPrName != "" {
				assert.Assert(t, got.GetName() == tt.wantPrName, "Pipelinerun hasn't been matched: "+got.GetName()+"!="+tt.wantPrName)
			}
			if tt.wantLog != "" {
				logmsg := log.TakeAll()
				assert.Assert(t, len(logmsg) > 0, "We didn't get any log message")
				assert.Assert(t, strings.Contains(logmsg[0].Message, tt.wantLog), logmsg[0].Message, tt.wantLog)
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
