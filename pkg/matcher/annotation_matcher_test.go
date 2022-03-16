package matcher

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	testnewrepo "github.com/openshift-pipelines/pipelines-as-code/pkg/test/repository"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestMatchPipelinerunAnnotationAndRepositories(t *testing.T) {
	cw := clockwork.NewFakeClock()
	pipelineTargetNSName := "pipeline-target-ns"
	pipelineTargetNS := &tektonv1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: pipelineTargetNSName,
			Annotations: map[string]string{
				filepath.Join(pipelinesascode.GroupName, onTargetNamespace):        targetNamespace,
				filepath.Join(pipelinesascode.GroupName, onEventAnnotation):        "[pull_request]",
				filepath.Join(pipelinesascode.GroupName, onTargetBranchAnnotation): fmt.Sprintf("[%s]", mainBranch),
				filepath.Join(pipelinesascode.GroupName, maxKeepRuns):              "2",
			},
		},
	}

	type args struct {
		pruns    []*tektonv1beta1.PipelineRun
		runevent info.Event
		data     testclient.Data
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
				pruns: []*tektonv1beta1.PipelineRun{pipelineTargetNS},
				runevent: info.Event{
					URL: targetURL, TriggerTarget: "pull_request", EventType: "pull_request",
					BaseBranch: mainBranch,
				},
				data: testclient.Data{
					Repositories: []*v1alpha1.Repository{
						testnewrepo.NewRepo(
							testnewrepo.RepoTestcreationOpts{
								Name:             "test-good",
								URL:              targetURL,
								InstallNamespace: targetNamespace,
							},
						),
					},
				},
			},
		},
		{
			name:       "match a repository on a CEL expression",
			wantPRName: pipelineTargetNSName,
			args: args{
				pruns: []*tektonv1beta1.PipelineRun{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: pipelineTargetNSName,
							Annotations: map[string]string{
								pipelinesascode.GroupName + "/" + onCelExpression: "event == \"pull_request" +
									"\" && target_branch == \"" + mainBranch + "\" && source_branch == \"unittests\"",
							},
						},
					},
				},
				runevent: info.Event{
					URL:           targetURL,
					TriggerTarget: "pull_request",
					EventType:     "pull_request",
					BaseBranch:    mainBranch,
					HeadBranch:    "unittests",
				},
				data: testclient.Data{
					Repositories: []*v1alpha1.Repository{
						testnewrepo.NewRepo(
							testnewrepo.RepoTestcreationOpts{
								Name:             "test-good",
								URL:              targetURL,
								InstallNamespace: targetNamespace,
							},
						),
					},
				},
			},
		},
		{
			name:    "bad CEL expresion",
			wantErr: true,
			args: args{
				pruns: []*tektonv1beta1.PipelineRun{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: pipelineTargetNSName,
							Annotations: map[string]string{
								filepath.Join(pipelinesascode.GroupName, onCelExpression): "BADDDDDDDx'ax\\\a",
							},
						},
					},
				},
				runevent: info.Event{
					URL:           targetURL,
					TriggerTarget: "pull_request",
					EventType:     "pull_request",
					BaseBranch:    mainBranch,
					HeadBranch:    "unittests",
				},
				data: testclient.Data{
					Repositories: []*v1alpha1.Repository{
						testnewrepo.NewRepo(
							testnewrepo.RepoTestcreationOpts{
								Name:             "test-good",
								URL:              targetURL,
								InstallNamespace: targetNamespace,
							},
						),
					},
				},
			},
		},
		{
			name:         "match same webhook on multiple repos takes the oldest one",
			wantPRName:   pipelineTargetNSName,
			wantRepoName: "test-oldest",
			args: args{
				pruns: []*tektonv1beta1.PipelineRun{pipelineTargetNS},
				runevent: info.Event{
					URL: targetURL, TriggerTarget: "pull_request", EventType: "pull_request",
					BaseBranch: mainBranch,
				},
				data: testclient.Data{
					Repositories: []*v1alpha1.Repository{
						testnewrepo.NewRepo(
							testnewrepo.RepoTestcreationOpts{
								Name:             "test-oldest",
								URL:              targetURL,
								InstallNamespace: targetNamespace,
								CreateTime:       metav1.Time{Time: cw.Now().Add(-55 * time.Minute)},
							},
						),
						testnewrepo.NewRepo(
							testnewrepo.RepoTestcreationOpts{
								Name:             "test-newest",
								URL:              targetURL,
								InstallNamespace: targetNamespace,
								CreateTime:       metav1.Time{Time: cw.Now().Add(-50 * time.Minute)},
							},
						),
					},
				},
			},
		},
		{
			name:    "error on only when on annotation",
			wantErr: true,
			args: args{
				pruns: []*tektonv1beta1.PipelineRun{
					{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: targetNamespace,
							Name:      "only-one-annotation",
							Annotations: map[string]string{
								pipelinesascode.GroupName + "/" + onEventAnnotation: "[pull_request]",
							},
						},
					},
				},
				runevent: info.Event{URL: targetURL, EventType: "pull_request", BaseBranch: mainBranch},
				data: testclient.Data{
					Repositories: []*v1alpha1.Repository{
						testnewrepo.NewRepo(
							testnewrepo.RepoTestcreationOpts{
								Name:             "test-oldest",
								URL:              targetURL,
								InstallNamespace: targetNamespace,
								CreateTime:       metav1.Time{Time: cw.Now().Add(-55 * time.Minute)},
							},
						),
					},
				},
			},
		},
		{
			name:    "error when no pac annotation has been set",
			wantErr: true,
			args: args{
				pruns: []*tektonv1beta1.PipelineRun{
					{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: targetNamespace,
							Name:      "no pac annotation",
							Annotations: map[string]string{
								"foo": "bar",
							},
						},
					},
				},
				runevent: info.Event{URL: targetURL, EventType: "pull_request", BaseBranch: mainBranch},
				data: testclient.Data{
					Repositories: []*v1alpha1.Repository{
						testnewrepo.NewRepo(
							testnewrepo.RepoTestcreationOpts{
								Name:             "test-oldest",
								URL:              targetURL,
								InstallNamespace: targetNamespace,
								CreateTime:       metav1.Time{Time: cw.Now().Add(-55 * time.Minute)},
							},
						),
					},
				},
			},
		},
		{
			name:    "error when pac annotation has been set but empty",
			wantErr: true,
			args: args{
				pruns: []*tektonv1beta1.PipelineRun{
					{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: targetNamespace,
							Name:      "no pac annotation",
							Annotations: map[string]string{
								filepath.Join(pipelinesascode.GroupName, onEventAnnotation):        "",
								filepath.Join(pipelinesascode.GroupName, onTargetBranchAnnotation): "",
							},
						},
					},
				},
				runevent: info.Event{URL: targetURL, EventType: "pull_request", BaseBranch: mainBranch},
				data: testclient.Data{
					Repositories: []*v1alpha1.Repository{
						testnewrepo.NewRepo(
							testnewrepo.RepoTestcreationOpts{
								Name:             "test-oldest",
								URL:              targetURL,
								InstallNamespace: targetNamespace,
								CreateTime:       metav1.Time{Time: cw.Now().Add(-55 * time.Minute)},
							},
						),
					},
				},
			},
		},
		{
			name:    "no match a repository with target NS",
			wantErr: true,
			wantLog: "matching a pipeline to event: URL",
			args: args{
				pruns: []*tektonv1beta1.PipelineRun{pipelineTargetNS},
				runevent: info.Event{
					URL: targetURL, TriggerTarget: "pull_request", EventType: "pull_request", BaseBranch: mainBranch,
				},
				data: testclient.Data{
					Repositories: []*v1alpha1.Repository{
						testnewrepo.NewRepo(
							testnewrepo.RepoTestcreationOpts{
								Name:             "test-good",
								URL:              targetURL,
								InstallNamespace: "otherNS",
							},
						),
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
			client := &params.Run{
				Clients: clients.Clients{PipelineAsCode: cs.PipelineAsCode, Log: logger},
				Info: info.Info{
					Event: &tt.args.runevent,
				},
			}
			got, repo, _, err := MatchPipelinerunByAnnotation(ctx,
				tt.args.pruns,
				client)

			if tt.wantErr {
				assert.Assert(t, err != nil, "We should have get an error")
			}

			if !tt.wantErr {
				assert.NilError(t, err)
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
				filepath.Join(pipelinesascode.GroupName, onEventAnnotation):        "[pull_request]",
				filepath.Join(pipelinesascode.GroupName, onTargetBranchAnnotation): "[main]",
			},
		},
	}

	pipelineOther := &tektonv1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipeline-other",
			Annotations: map[string]string{
				filepath.Join(pipelinesascode.GroupName, onEventAnnotation):        "[pull_request]",
				filepath.Join(pipelinesascode.GroupName, onTargetBranchAnnotation): "[main]",
			},
		},
	}

	observer, log := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()

	type args struct {
		pruns    []*tektonv1beta1.PipelineRun
		runevent info.Event
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
				pruns:    []*tektonv1beta1.PipelineRun{pipelineGood},
				runevent: info.Event{TriggerTarget: "pull_request", EventType: "pull_request", BaseBranch: "main"},
			},
			wantErr:    false,
			wantPrName: "pipeline-good",
		},
		{
			name: "first-one-match-with-two-good-ones",
			args: args{
				pruns:    []*tektonv1beta1.PipelineRun{pipelineGood, pipelineOther},
				runevent: info.Event{TriggerTarget: "pull_request", EventType: "pull_request", BaseBranch: "main"},
			},
			wantErr:    false,
			wantPrName: "pipeline-good",
		},
		{
			name: "no-match-on-event",
			args: args{
				pruns:    []*tektonv1beta1.PipelineRun{pipelineGood, pipelineOther},
				runevent: info.Event{TriggerTarget: "push", EventType: "push", BaseBranch: "main"},
			},
			wantErr: true,
		},
		{
			name: "no-match-on-target-branch",
			args: args{
				pruns:    []*tektonv1beta1.PipelineRun{pipelineGood, pipelineOther},
				runevent: info.Event{TriggerTarget: "pull_request", EventType: "pull_request", BaseBranch: "other"},
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
				runevent: info.Event{TriggerTarget: "push", EventType: "push", BaseBranch: "main"},
			},
			wantErr: true,
		},
		{
			name: "single-event-annotation",
			args: args{
				runevent: info.Event{TriggerTarget: "pull_request", EventType: "pull_request", BaseBranch: "main"},
				pruns: []*tektonv1beta1.PipelineRun{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "single-event-annotation",
							Annotations: map[string]string{
								filepath.Join(pipelinesascode.GroupName, onEventAnnotation):        "pull_request",
								filepath.Join(pipelinesascode.GroupName, onTargetBranchAnnotation): "[main]",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "single-target-branch-annotation",
			args: args{
				runevent: info.Event{TriggerTarget: "pull_request", EventType: "pull_request", BaseBranch: "main"},
				pruns: []*tektonv1beta1.PipelineRun{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "single-target-branch-annotation",
							Annotations: map[string]string{
								filepath.Join(pipelinesascode.GroupName, onEventAnnotation):        "[pull_request]",
								filepath.Join(pipelinesascode.GroupName, onTargetBranchAnnotation): "main",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "empty-annotation",
			args: args{
				runevent: info.Event{TriggerTarget: "pull_request", EventType: "pull_request", BaseBranch: "main"},
				pruns: []*tektonv1beta1.PipelineRun{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "bad-target-branch-annotation",
							Annotations: map[string]string{
								filepath.Join(pipelinesascode.GroupName, onEventAnnotation):        "[]",
								filepath.Join(pipelinesascode.GroupName, onTargetBranchAnnotation): "[]",
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
				runevent: info.Event{TriggerTarget: "push", EventType: "push", BaseBranch: "refs/heads/main"},
				pruns: []*tektonv1beta1.PipelineRun{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "branch-matching",
							Annotations: map[string]string{
								filepath.Join(pipelinesascode.GroupName, onEventAnnotation):        "[push]",
								filepath.Join(pipelinesascode.GroupName, onTargetBranchAnnotation): "[main]",
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
				runevent: info.Event{
					TriggerTarget: "push", EventType: "push",
					BaseBranch: "refs/heads/main/foobar",
				},
				pruns: []*tektonv1beta1.PipelineRun{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "branch-base-matching-not-compare",
							Annotations: map[string]string{
								filepath.Join(pipelinesascode.GroupName, onEventAnnotation):        "[push]",
								filepath.Join(pipelinesascode.GroupName, onTargetBranchAnnotation): "[main]",
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
				runevent: info.Event{TriggerTarget: "push", EventType: "push", BaseBranch: "refs/heads/main"},
				pruns: []*tektonv1beta1.PipelineRun{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "branch-base-matching-not-compare",
							Annotations: map[string]string{
								filepath.Join(pipelinesascode.GroupName, onEventAnnotation):        "[push]",
								filepath.Join(pipelinesascode.GroupName, onTargetBranchAnnotation): "[refs/heads/*]",
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
			cs := &params.Run{
				Clients: clients.Clients{
					Log: logger,
				},
				Info: info.Info{
					Event: &tt.args.runevent,
				},
			}
			got, _, _, err := MatchPipelinerunByAnnotation(ctx, tt.args.pruns, cs)
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
			name: "get-annotation-string",
			args: args{
				annotation: "foo",
			},
			want:    []string{"foo"},
			wantErr: false,
		},
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
			name: "get-annotation-multiple-string-bad-syntax",
			args: args{
				annotation: "foo, bar",
			},
			wantErr: true,
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
