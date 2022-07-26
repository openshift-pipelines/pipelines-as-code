package matcher

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v45/github"
	"github.com/jonboulle/clockwork"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	ghprovider "github.com/openshift-pipelines/pipelines-as-code/pkg/provider/github"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	ghtesthelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test/github"
	testnewrepo "github.com/openshift-pipelines/pipelines-as-code/pkg/test/repository"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func replyGhListFiles(t *testing.T, mux *http.ServeMux, url string, commitFiles []*github.CommitFile) {
	t.Helper()
	mux.HandleFunc(url, func(w http.ResponseWriter, r *http.Request) {
		jeez, err := json.Marshal(commitFiles)
		assert.NilError(t, err)
		_, _ = w.Write(jeez)
	})
}

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
		fileChanged []*github.CommitFile
		pruns       []*tektonv1beta1.PipelineRun
		runevent    info.Event
		data        testclient.Data
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
			name:       "cel/match source/target",
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
			name:       "cel/match path by glob",
			wantPRName: pipelineTargetNSName,
			args: args{
				fileChanged: []*github.CommitFile{
					{
						Filename: github.String(".tekton/pull_request.yaml"),
					},
				},
				pruns: []*tektonv1beta1.PipelineRun{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: pipelineTargetNSName,
							Annotations: map[string]string{
								pipelinesascode.GroupName + "/" + onCelExpression: "\".tekton/*yaml\"." +
									"pathChanged()",
							},
						},
					},
				},
				runevent: info.Event{
					URL:               targetURL,
					TriggerTarget:     "pull_request",
					EventType:         "pull_request",
					BaseBranch:        mainBranch,
					HeadBranch:        "unittests",
					PullRequestNumber: 1000,
					Organization:      "mylittle",
					Repository:        "pony",
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
			name:    "cel/no match path by glob",
			wantErr: true,
			args: args{
				fileChanged: []*github.CommitFile{
					{
						Filename: github.String(".tekton/foo.json"),
					},
				},
				pruns: []*tektonv1beta1.PipelineRun{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: pipelineTargetNSName,
							Annotations: map[string]string{
								pipelinesascode.GroupName + "/" + onCelExpression: "\".tekton/*yaml\"." +
									"pathChanged()",
							},
						},
					},
				},
				runevent: info.Event{
					URL:               targetURL,
					TriggerTarget:     "pull_request",
					EventType:         "pull_request",
					BaseBranch:        mainBranch,
					HeadBranch:        "unittests",
					PullRequestNumber: 1000,
					Organization:      "mylittle",
					Repository:        "pony",
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
			name:       "cel/match by direct path",
			wantPRName: pipelineTargetNSName,
			args: args{
				fileChanged: []*github.CommitFile{
					{
						Filename: github.String(".tekton/pull_request.yaml"),
					},
				},
				pruns: []*tektonv1beta1.PipelineRun{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: pipelineTargetNSName,
							Annotations: map[string]string{
								pipelinesascode.GroupName + "/" + onCelExpression: "\".tekton/pull_request.yaml\"." +
									"pathChanged()",
							},
						},
					},
				},
				runevent: info.Event{
					URL:               targetURL,
					TriggerTarget:     "pull_request",
					EventType:         "pull_request",
					BaseBranch:        mainBranch,
					HeadBranch:        "unittests",
					PullRequestNumber: 1000,
					Organization:      "mylittle",
					Repository:        "pony",
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
			name:    "match TargetPipelineRun",
			wantErr: false,
			args: args{
				pruns: []*tektonv1beta1.PipelineRun{
					{
						ObjectMeta: metav1.ObjectMeta{
							GenerateName: fmt.Sprintf("%s-", pipelineTargetNSName),
						},
					},
				},
				runevent: info.Event{
					URL:               targetURL,
					TargetPipelineRun: pipelineTargetNSName,
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
			name:    "cel/bad expression",
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
			wantLog: "matching pipelineruns to event: URL",
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
				Clients: clients.Clients{PipelineAsCode: cs.PipelineAsCode},
				Info:    info.Info{},
			}

			fakeclient, mux, ghTestServerURL, teardown := ghtesthelper.SetupGH()
			defer teardown()
			vcx := &ghprovider.Provider{
				Client: fakeclient,
				Token:  github.String("None"),
				Logger: logger,
			}
			if len(tt.args.fileChanged) > 0 {
				url := fmt.Sprintf("/repos/%s/%s/pulls/%d/files", tt.args.runevent.Organization,
					tt.args.runevent.Repository, tt.args.runevent.PullRequestNumber)
				replyGhListFiles(t, mux, url, tt.args.fileChanged)
			}

			tt.args.runevent.Provider = &info.Provider{
				URL:   ghTestServerURL,
				Token: "NONE",
			}
			matches, err := MatchPipelinerunByAnnotation(ctx, logger,
				tt.args.pruns,
				client, &tt.args.runevent, vcx)

			if tt.wantErr {
				assert.Assert(t, err != nil, "We should have get an error")
			}

			if !tt.wantErr {
				assert.NilError(t, err)
			}

			if tt.wantRepoName != "" {
				assert.Assert(t, tt.wantRepoName == matches[0].Repo.GetName())
			}
			if tt.wantPRName != "" {
				assert.Assert(t, tt.wantPRName == matches[0].PipelineRun.GetName())
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
				Clients: clients.Clients{},
				Info:    info.Info{},
			}
			matches, err := MatchPipelinerunByAnnotation(ctx, logger, tt.args.pruns, cs, &tt.args.runevent, &ghprovider.Provider{})
			if (err != nil) != tt.wantErr {
				t.Errorf("MatchPipelinerunByAnnotation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantPrName != "" {
				assert.Assert(t, matches[0].PipelineRun.GetName() == tt.wantPrName, "Pipelinerun hasn't been matched: %+v",
					matches[0].PipelineRun.GetName(), tt.wantPrName)
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
