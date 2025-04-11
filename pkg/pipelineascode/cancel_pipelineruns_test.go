package pipelineascode

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/google/go-github/v68/github"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	knativeduckv1 "knative.dev/pkg/apis/duck/v1"
	rtesting "knative.dev/pkg/reconciler/testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/formatting"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/opscomments"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
)

var (
	pullReqNumber = 11
	fooRepo       = &v1alpha1.Repository{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
			Name:      "foo",
		},
		Spec: v1alpha1.RepositorySpec{
			URL: "https://github.com/fooorg/foo",
		},
	}
	fooRepoLabelsForPush = map[string]string{
		keys.URLRepository: formatting.CleanValueKubernetes("foo"),
		keys.SHA:           formatting.CleanValueKubernetes("foosha"),
	}
	fooRepoLabels = map[string]string{
		keys.OriginalPRName: "pr-foo",
		keys.URLRepository:  formatting.CleanValueKubernetes("foo"),
		keys.SHA:            formatting.CleanValueKubernetes("foosha"),
		keys.PullRequest:    strconv.Itoa(pullReqNumber),
		keys.EventType:      string(triggertype.PullRequest),
	}
	fooRepoAnnotations = map[string]string{
		keys.URLRepository: "foo",
		keys.SHA:           "foosha",
		keys.PullRequest:   strconv.Itoa(pullReqNumber),
		keys.Repository:    "foo",
	}
	fooRepoLabelsPrFooAbc = map[string]string{
		keys.URLRepository:  formatting.CleanValueKubernetes("foo"),
		keys.SHA:            formatting.CleanValueKubernetes("foosha"),
		keys.PullRequest:    strconv.Itoa(pullReqNumber),
		keys.OriginalPRName: "pr-foo-abc",
		keys.Repository:     "foo",
	}
	fooRepoAnnotationsPrFooAbc = map[string]string{
		keys.URLRepository:  "foo",
		keys.SHA:            "foosha",
		keys.PullRequest:    strconv.Itoa(pullReqNumber),
		keys.OriginalPRName: "pr-foo-abc",
		keys.Repository:     "foo",
	}
)

func TestCancelPipelinerunOpsComment(t *testing.T) {
	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()
	tests := []struct {
		name                  string
		event                 *info.Event
		repo                  *v1alpha1.Repository
		pipelineRuns          []*pipelinev1.PipelineRun
		cancelledPipelineRuns map[string]bool
	}{
		{
			name: "cancel running",
			event: &info.Event{
				Repository:        "foo",
				SHA:               "foosha",
				TriggerTarget:     "pull_request",
				PullRequestNumber: pullReqNumber,
				State: info.State{
					CancelPipelineRuns: true,
				},
			},
			pipelineRuns: []*pipelinev1.PipelineRun{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pr-foo",
						Namespace: "foo",
						Labels:    fooRepoLabels,
					},
					Spec: pipelinev1.PipelineRunSpec{},
				},
			},
			repo: fooRepo,
			cancelledPipelineRuns: map[string]bool{
				"pr-foo": true,
			},
		},
		{
			name: "no pipelineruns found",
			event: &info.Event{
				Repository:        "foo",
				SHA:               "foosha",
				TriggerTarget:     "pull_request",
				PullRequestNumber: pullReqNumber,
				State: info.State{
					CancelPipelineRuns: true,
				},
			},
			pipelineRuns:          []*pipelinev1.PipelineRun{},
			repo:                  fooRepo,
			cancelledPipelineRuns: map[string]bool{},
		},
		{
			name: "cancel a specific run",
			event: &info.Event{
				Repository:        "foo",
				SHA:               "foosha",
				TriggerTarget:     "pull_request",
				PullRequestNumber: pullReqNumber,
				State: info.State{
					CancelPipelineRuns:      true,
					TargetCancelPipelineRun: "pr-foo-abc",
				},
			},
			pipelineRuns: []*pipelinev1.PipelineRun{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "pr-foo",
						Namespace:   "foo",
						Labels:      fooRepoLabels,
						Annotations: fooRepoAnnotations,
					},
					Spec: pipelinev1.PipelineRunSpec{},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "pr-foo-abc-123",
						Namespace:   "foo",
						Labels:      fooRepoLabelsPrFooAbc,
						Annotations: fooRepoAnnotationsPrFooAbc,
					},
					Spec: pipelinev1.PipelineRunSpec{},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "pr-foo-pqr",
						Namespace:   "foo",
						Labels:      fooRepoLabels,
						Annotations: fooRepoAnnotations,
					},
					Spec: pipelinev1.PipelineRunSpec{},
				},
			},
			repo: fooRepo,
			cancelledPipelineRuns: map[string]bool{
				"pr-foo-abc-123": true,
			},
		},
		{
			name: "cancelling a done pipelinerun or already cancelled pipelinerun",
			event: &info.Event{
				Repository:        "foo",
				SHA:               "foosha",
				TriggerTarget:     "pull_request",
				PullRequestNumber: pullReqNumber,
				State: info.State{
					CancelPipelineRuns: true,
				},
			},
			pipelineRuns: []*pipelinev1.PipelineRun{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pr-foo",
						Namespace: "foo",
						Labels:    fooRepoLabels,
					},
					Spec: pipelinev1.PipelineRunSpec{},
					Status: pipelinev1.PipelineRunStatus{
						Status: knativeduckv1.Status{
							Conditions: knativeduckv1.Conditions{
								apis.Condition{
									Type:   apis.ConditionSucceeded,
									Status: corev1.ConditionTrue,
								},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pr-foo-1",
						Namespace: "foo",
						Labels:    fooRepoLabels,
					},
					Spec: pipelinev1.PipelineRunSpec{
						Status: pipelinev1.PipelineRunSpecStatusStoppedRunFinally,
					},
				},
			},
			repo:                  fooRepo,
			cancelledPipelineRuns: map[string]bool{},
		},
		{
			name: "cancel running for push event",
			event: &info.Event{
				Repository:    "foo",
				SHA:           "foosha",
				TriggerTarget: "push",
				State: info.State{
					CancelPipelineRuns: true,
				},
			},
			pipelineRuns: []*pipelinev1.PipelineRun{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pr-foo",
						Namespace: "foo",
						Labels:    fooRepoLabelsForPush,
					},
					Spec: pipelinev1.PipelineRunSpec{},
				},
			},
			repo: fooRepo,
			cancelledPipelineRuns: map[string]bool{
				"pr-foo": true,
			},
		},
		{
			name: "cancel a specific run for push event",
			event: &info.Event{
				Repository:    "foo",
				SHA:           "foosha",
				TriggerTarget: "push",
				State: info.State{
					CancelPipelineRuns:      true,
					TargetCancelPipelineRun: "pr-foo-abc",
				},
			},
			pipelineRuns: []*pipelinev1.PipelineRun{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "pr-foo",
						Namespace:   "foo",
						Labels:      fooRepoLabelsForPush,
						Annotations: fooRepoAnnotations,
					},
					Spec: pipelinev1.PipelineRunSpec{},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "pr-foo-abc-123",
						Namespace:   "foo",
						Labels:      fooRepoLabelsPrFooAbc,
						Annotations: fooRepoAnnotationsPrFooAbc,
					},
					Spec: pipelinev1.PipelineRunSpec{},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "pr-foo-pqr",
						Namespace:   "foo",
						Labels:      fooRepoLabelsForPush,
						Annotations: fooRepoAnnotations,
					},
					Spec: pipelinev1.PipelineRunSpec{},
				},
			},
			repo: fooRepo,
			cancelledPipelineRuns: map[string]bool{
				"pr-foo-abc-123": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)

			tdata := testclient.Data{
				PipelineRuns: tt.pipelineRuns,
			}
			stdata, _ := testclient.SeedTestData(t, ctx, tdata)
			cs := &params.Run{
				Clients: clients.Clients{
					Log:    logger,
					Tekton: stdata.Pipeline,
					Kube:   stdata.Kube,
				},
			}
			pac := NewPacs(tt.event, nil, cs, &info.PacOpts{}, nil, logger, nil)
			err := pac.cancelPipelineRunsOpsComment(ctx, tt.repo)
			assert.NilError(t, err)

			got, err := cs.Clients.Tekton.TektonV1().PipelineRuns("foo").List(ctx, metav1.ListOptions{})
			assert.NilError(t, err)

			for _, pr := range got.Items {
				// from the list only the ones which are in cancelled map should have cancel status
				if _, ok := tt.cancelledPipelineRuns[pr.Name]; ok {
					assert.Equal(t, string(pr.Spec.Status), pipelinev1.PipelineRunSpecStatusCancelledRunFinally)
					continue
				}
				assert.Assert(t, string(pr.Spec.Status) != pipelinev1.PipelineRunSpecStatusCancelledRunFinally)
			}
		})
	}
}

func TestCancelInProgressMatchingPR(t *testing.T) {
	observer, catcher := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()
	tests := []struct {
		name                  string
		nocancelcheck         bool
		event                 *info.Event
		repo                  *v1alpha1.Repository
		pipelineRuns          []*pipelinev1.PipelineRun
		cancelledPipelineRuns map[string]bool
		wantErrString         string
		wantLog               string
	}{
		{
			name:         "skipped/no pr",
			pipelineRuns: nil,
		},
		{
			name: "skipped/no cancel in progress annotations",
			pipelineRuns: []*pipelinev1.PipelineRun{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pr-foo",
						Namespace: "foo",
						Labels:    fooRepoLabels,
					},
				},
			},
		},
		{
			name: "skipped/no original pr name",
			pipelineRuns: []*pipelinev1.PipelineRun{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pr-foo",
						Namespace: "foo",
						Labels:    map[string]string{},
						Annotations: map[string]string{
							keys.CancelInProgress: "true",
						},
					},
				},
			},
		},
		{
			name: "skip/finished pr",
			event: &info.Event{
				Repository:        "foo",
				SHA:               "foosha",
				HeadBranch:        "head",
				EventType:         string(triggertype.PullRequest),
				TriggerTarget:     triggertype.PullRequest,
				PullRequestNumber: pullReqNumber,
			},
			pipelineRuns: []*pipelinev1.PipelineRun{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pr-foo",
						Namespace: "foo",
						Labels:    fooRepoLabels,
						Annotations: map[string]string{
							keys.CancelInProgress: "true",
							keys.OriginalPRName:   "pr-foo",
							keys.Repository:       "foo",
							keys.SourceBranch:     "head",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pr-foo-1",
						Namespace: "foo",
						Labels:    fooRepoLabels,
						Annotations: map[string]string{
							keys.CancelInProgress: "true", keys.OriginalPRName: "pr-foo",
							keys.Repository:   "foo",
							keys.SourceBranch: "head",
						},
					},
					Status: pipelinev1.PipelineRunStatus{
						Status: knativeduckv1.Status{
							Conditions: knativeduckv1.Conditions{
								apis.Condition{
									Type:   apis.ConditionSucceeded,
									Status: corev1.ConditionTrue,
								},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pr-foo-2",
						Namespace: "foo",
						Labels:    fooRepoLabels,
						Annotations: map[string]string{
							keys.CancelInProgress: "true", keys.OriginalPRName: "pr-foo",
							keys.Repository:   "foo",
							keys.SourceBranch: "head",
						},
					},
				},
			},
			repo: fooRepo,
			cancelledPipelineRuns: map[string]bool{
				"pr-foo-2": true,
			},
		},
		{
			name:          "skip/cancelled pr",
			nocancelcheck: true,
			event: &info.Event{
				Repository:        "foo",
				SHA:               "foosha",
				HeadBranch:        "head",
				EventType:         string(triggertype.PullRequest),
				TriggerTarget:     triggertype.PullRequest,
				PullRequestNumber: pullReqNumber,
			},
			pipelineRuns: []*pipelinev1.PipelineRun{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pr-foo",
						Namespace: "foo",
						Labels:    fooRepoLabels,
						Annotations: map[string]string{
							keys.CancelInProgress: "true",
							keys.OriginalPRName:   "pr-foo",
							keys.Repository:       "foo",
							keys.SourceBranch:     "head",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pr-cancelled",
						Namespace: "foo",
						Labels:    fooRepoLabels,
						Annotations: map[string]string{
							keys.CancelInProgress: "true", keys.OriginalPRName: "pr-foo",
							keys.Repository:   "foo",
							keys.SourceBranch: "head",
						},
					},
					Status: pipelinev1.PipelineRunStatus{
						Status: knativeduckv1.Status{
							Conditions: knativeduckv1.Conditions{
								apis.Condition{
									Type:   apis.ConditionSucceeded,
									Status: corev1.ConditionTrue,
									Reason: pipelinev1.PipelineRunSpecStatusCancelled,
								},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pr-foo-fou",
						Namespace: "foo",
						Labels:    fooRepoLabels,
						Annotations: map[string]string{
							keys.CancelInProgress: "true", keys.OriginalPRName: "pr-foo",
							keys.Repository:   "foo",
							keys.SourceBranch: "head",
						},
					},
				},
			},
			repo: fooRepo,
			cancelledPipelineRuns: map[string]bool{
				"pr-foo-2": true,
			},
		},
		{
			name: "match/cancel in progress",
			event: &info.Event{
				Repository:        "foo",
				SHA:               "foosha",
				HeadBranch:        "head",
				EventType:         string(triggertype.PullRequest),
				TriggerTarget:     triggertype.PullRequest,
				PullRequestNumber: pullReqNumber,
			},
			pipelineRuns: []*pipelinev1.PipelineRun{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pr-foo",
						Namespace: "foo",
						Labels:    fooRepoLabels,
						Annotations: map[string]string{
							keys.CancelInProgress: "true",
							keys.OriginalPRName:   "pr-foo",
							keys.Repository:       "foo",
							keys.SourceBranch:     "head",
						},
					},
					Spec: pipelinev1.PipelineRunSpec{},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pr-foo-1",
						Namespace: "foo",
						Labels:    fooRepoLabels,
						Annotations: map[string]string{
							keys.CancelInProgress: "true",
							keys.OriginalPRName:   "pr-foo",
							keys.Repository:       "foo",
							keys.SourceBranch:     "head",
						},
					},
					Spec: pipelinev1.PipelineRunSpec{},
				},
			},
			repo: fooRepo,
			cancelledPipelineRuns: map[string]bool{
				"pr-foo-1": true,
			},
			wantLog: "cancel-in-progress: cancelling pipelinerun foo/",
		},
		{
			name: "match/cancel in progress on PipelineRun generateName",
			event: &info.Event{
				Repository:        "foo",
				SHA:               "foosha",
				HeadBranch:        "head",
				EventType:         string(triggertype.PullRequest),
				TriggerTarget:     triggertype.PullRequest,
				PullRequestNumber: pullReqNumber,
			},
			pipelineRuns: []*pipelinev1.PipelineRun{
				{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "pr-foo-",
						Name:         "pr-foo-1",
						Namespace:    "foo",
						Labels:       fooRepoLabels,
						Annotations: map[string]string{
							keys.CancelInProgress: "true",
							keys.OriginalPRName:   "pr-foo",
							keys.Repository:       "foo",
							keys.SourceBranch:     "head",
						},
					},
					Spec: pipelinev1.PipelineRunSpec{},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "pr-foo-",
						Name:         "pr-foo-2",
						Namespace:    "foo",
						Labels:       fooRepoLabels,
						Annotations: map[string]string{
							keys.CancelInProgress: "true",
							keys.OriginalPRName:   "pr-foo",
							keys.Repository:       "foo",
							keys.SourceBranch:     "head",
						},
					},
					Spec: pipelinev1.PipelineRunSpec{},
				},
			},
			repo: fooRepo,
			cancelledPipelineRuns: map[string]bool{
				"pr-foo-2": true,
			},
			wantLog: "cancel-in-progress: cancelling pipelinerun foo/pr-foo-2",
		},
		{
			name: "match/cancel in progress from /retest",
			event: &info.Event{
				Repository:        "foo",
				SHA:               "foosha",
				HeadBranch:        "head",
				EventType:         string(triggertype.PullRequest),
				TriggerTarget:     triggertype.PullRequest,
				PullRequestNumber: pullReqNumber,
			},
			pipelineRuns: []*pipelinev1.PipelineRun{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pr-foo",
						Namespace: "foo",
						Labels:    fooRepoLabels,
						Annotations: map[string]string{
							keys.CancelInProgress: "true",
							keys.OriginalPRName:   "pr-foo",
							keys.Repository:       "foo",
							keys.SourceBranch:     "head",
						},
					},
					Spec: pipelinev1.PipelineRunSpec{},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pr-foo-1",
						Namespace: "foo",
						Labels: map[string]string{
							keys.OriginalPRName: "pr-foo",
							keys.URLRepository:  formatting.CleanValueKubernetes("foo"),
							keys.SHA:            formatting.CleanValueKubernetes("foosha"),
							keys.PullRequest:    strconv.Itoa(pullReqNumber),
							keys.EventType:      string(opscomments.RetestAllCommentEventType),
						},
						Annotations: map[string]string{
							keys.CancelInProgress: "true", keys.OriginalPRName: "pr-foo",
							keys.Repository:   "foo",
							keys.SourceBranch: "head",
						},
					},
					Spec: pipelinev1.PipelineRunSpec{},
				},
			},
			repo: fooRepo,
			cancelledPipelineRuns: map[string]bool{
				"pr-foo-1": true,
			},
			wantLog: "cancel-in-progress: cancelling pipelinerun foo/pr-foo-1",
		},
		{
			name: "match/cancel in progress exclude not belonging to same push branch",
			event: &info.Event{
				Repository:    "foo",
				SHA:           "foosha",
				HeadBranch:    "head",
				EventType:     string(triggertype.Push),
				TriggerTarget: triggertype.Push,
			},
			pipelineRuns: []*pipelinev1.PipelineRun{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pr-foo",
						Namespace: "foo",
						Labels: map[string]string{
							keys.OriginalPRName: "pr-foo",
							keys.URLRepository:  formatting.CleanValueKubernetes("foo"),
							keys.SHA:            formatting.CleanValueKubernetes("foosha"),
							keys.EventType:      string(triggertype.Push),
						},
						Annotations: map[string]string{
							keys.CancelInProgress: "true",
							keys.OriginalPRName:   "pr-foo",
							keys.Repository:       "foo",
							keys.SourceBranch:     "head",
						},
					},
					Spec: pipelinev1.PipelineRunSpec{},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pr-foo-1",
						Namespace: "foo",
						Labels: map[string]string{
							keys.OriginalPRName: "pr-foo",
							keys.URLRepository:  formatting.CleanValueKubernetes("foo"),
							keys.SHA:            formatting.CleanValueKubernetes("foosha"),
							keys.EventType:      string(triggertype.Push),
						},
						Annotations: map[string]string{
							keys.CancelInProgress: "true", keys.OriginalPRName: "pr-foo",
							keys.Repository:   "foo",
							keys.SourceBranch: "anotherhead",
						},
					},
					Spec: pipelinev1.PipelineRunSpec{},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pr-foo-2",
						Namespace: "foo",
						Labels: map[string]string{
							keys.OriginalPRName: "pr-foo",
							keys.URLRepository:  formatting.CleanValueKubernetes("foo"),
							keys.SHA:            formatting.CleanValueKubernetes("foosha"),
							keys.EventType:      string(triggertype.Push),
						},
						Annotations: map[string]string{
							keys.CancelInProgress: "true", keys.OriginalPRName: "pr-foo",
							keys.Repository:   "foo",
							keys.SourceBranch: "head",
						},
					},
					Spec: pipelinev1.PipelineRunSpec{},
				},
			},
			repo: fooRepo,
			cancelledPipelineRuns: map[string]bool{
				"pr-foo-2": true,
			},
			wantLog: "skipping pipelinerun foo/pr-foo-1 as it is not from the same branch",
		},
		{
			name: "match/cancel in progress exclude not belonging to same pr",
			event: &info.Event{
				Repository:        "foo",
				SHA:               "foosha",
				HeadBranch:        "head",
				EventType:         string(triggertype.PullRequest),
				TriggerTarget:     triggertype.PullRequest,
				PullRequestNumber: pullReqNumber,
			},
			pipelineRuns: []*pipelinev1.PipelineRun{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pr-foo",
						Namespace: "foo",
						Labels: map[string]string{
							keys.OriginalPRName: "pr-foo",
							keys.URLRepository:  formatting.CleanValueKubernetes("foo"),
							keys.SHA:            formatting.CleanValueKubernetes("foosha"),
							keys.PullRequest:    strconv.Itoa(pullReqNumber),
							keys.EventType:      string(triggertype.PullRequest),
						},
						Annotations: map[string]string{
							keys.CancelInProgress: "true",
							keys.OriginalPRName:   "pr-foo",
							keys.Repository:       "foo",
							keys.SourceBranch:     "head",
						},
					},
					Spec: pipelinev1.PipelineRunSpec{},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pr-foo-1",
						Namespace: "foo",
						Labels: map[string]string{
							keys.OriginalPRName: "pr-foo",
							keys.URLRepository:  formatting.CleanValueKubernetes("foo"),
							keys.SHA:            formatting.CleanValueKubernetes("foosha"),
							keys.PullRequest:    strconv.Itoa(10),
							keys.EventType:      string(triggertype.PullRequest),
						},
						Annotations: map[string]string{
							keys.CancelInProgress: "true", keys.OriginalPRName: "pr-foo",
							keys.Repository:   "foo",
							keys.SourceBranch: "head",
						},
					},
					Spec: pipelinev1.PipelineRunSpec{},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pr-foo-2",
						Namespace: "foo",
						Labels: map[string]string{
							keys.OriginalPRName: "pr-foo",
							keys.URLRepository:  formatting.CleanValueKubernetes("foo"),
							keys.SHA:            formatting.CleanValueKubernetes("foosha"),
							keys.PullRequest:    strconv.Itoa(pullReqNumber),
							keys.EventType:      string(triggertype.PullRequest),
						},
						Annotations: map[string]string{
							keys.CancelInProgress: "true", keys.OriginalPRName: "pr-foo",
							keys.Repository:   "foo",
							keys.SourceBranch: "head",
						},
					},
					Spec: pipelinev1.PipelineRunSpec{},
				},
			},
			repo: fooRepo,
			cancelledPipelineRuns: map[string]bool{
				"pr-foo-2": true,
			},
			wantLog: "cancel-in-progress: cancelling pipelinerun foo/",
		},
		{
			name: "skip/cancel in progress with concurrency limit",
			event: &info.Event{
				Repository: "foo",
				SHA:        "foosha",
			},
			pipelineRuns: []*pipelinev1.PipelineRun{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "pr-foo",
						Namespace:   "foo",
						Labels:      fooRepoLabels,
						Annotations: map[string]string{keys.CancelInProgress: "true", keys.OriginalPRName: "pr-foo", keys.Repository: "foo"},
					},
					Spec: pipelinev1.PipelineRunSpec{},
				},
			},
			repo: &v1alpha1.Repository{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "foo",
				},
				Spec: v1alpha1.RepositorySpec{
					URL:              "https://github.com/fooorg/foo",
					ConcurrencyLimit: github.Ptr(1),
				},
			},
			cancelledPipelineRuns: map[string]bool{},
			wantErrString:         "cancel in progress is not supported with concurrency limit",
			wantLog:               "cancel-in-progress: cancelling pipelinerun foo/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)

			tdata := testclient.Data{
				PipelineRuns: tt.pipelineRuns,
			}
			stdata, _ := testclient.SeedTestData(t, ctx, tdata)
			cs := &params.Run{
				Clients: clients.Clients{
					Log:    logger,
					Tekton: stdata.Pipeline,
					Kube:   stdata.Kube,
				},
			}
			pac := NewPacs(tt.event, nil, cs, &info.PacOpts{}, nil, logger, nil)
			var firstPr *pipelinev1.PipelineRun
			if len(tt.pipelineRuns) > 0 {
				firstPr = tt.pipelineRuns[0]
			}
			err := pac.cancelInProgressMatchingPR(ctx, firstPr, tt.repo)
			if tt.wantErrString != "" {
				assert.ErrorContains(t, err, tt.wantErrString)
				return
			}
			assert.NilError(t, err)

			// the fake k8 test library don't set cancellation status, so we can't check the status :\
			got, err := cs.Clients.Tekton.TektonV1().PipelineRuns("foo").List(ctx, metav1.ListOptions{})
			assert.NilError(t, err)

			for _, pr := range got.Items {
				// from the list only the ones which are in cancelled map should have cancel status
				if _, ok := tt.cancelledPipelineRuns[pr.Name]; ok {
					assert.Equal(t, string(pr.Spec.Status), pipelinev1.PipelineRunSpecStatusCancelledRunFinally, pr.GetName())
					continue
				}
				if !tt.nocancelcheck {
					assert.Assert(t, string(pr.Spec.Status) != pipelinev1.PipelineRunSpecStatusCancelledRunFinally)
				}
			}

			if tt.wantLog != "" {
				assert.Assert(t, len(catcher.FilterMessageSnippet(tt.wantLog).TakeAll()) > 0, fmt.Sprintf("could not find log message: got %+v", catcher.TakeAll()))
			}
		})
	}
}

func TestCancelAllInProgressBelongingToClosedPullRequest(t *testing.T) {
	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()

	tests := []struct {
		name                  string
		event                 *info.Event
		repo                  *v1alpha1.Repository
		pipelineRuns          []*pipelinev1.PipelineRun
		cancelledPipelineRuns map[string]bool
	}{
		{
			name: "cancel all in progress PipelineRuns with annotation set to true",
			event: &info.Event{
				Repository:        "foo",
				TriggerTarget:     "pull_request",
				PullRequestNumber: pullReqNumber,
			},
			repo: fooRepo,
			pipelineRuns: []*pipelinev1.PipelineRun{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pr-foo-1",
						Namespace: "foo",
						Labels: map[string]string{
							keys.OriginalPRName:   "pr-foo",
							keys.URLRepository:    formatting.CleanValueKubernetes("foo"),
							keys.SHA:              formatting.CleanValueKubernetes("foosha"),
							keys.PullRequest:      strconv.Itoa(pullReqNumber),
							keys.EventType:        string(triggertype.PullRequest),
							keys.CancelInProgress: "true",
						},
					},
					Spec: pipelinev1.PipelineRunSpec{},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pr-foo-2",
						Namespace: "foo",
						Labels: map[string]string{
							keys.OriginalPRName:   "pr-foo",
							keys.URLRepository:    formatting.CleanValueKubernetes("foo"),
							keys.SHA:              formatting.CleanValueKubernetes("foosha"),
							keys.PullRequest:      strconv.Itoa(pullReqNumber),
							keys.EventType:        string(triggertype.PullRequest),
							keys.CancelInProgress: "true",
						},
					},
					Spec: pipelinev1.PipelineRunSpec{},
				},
			},
			cancelledPipelineRuns: map[string]bool{
				"pr-foo-1": true,
				"pr-foo-2": true,
			},
		},
		{
			name: "cancel all in progress PipelineRuns with annotation set to false",
			event: &info.Event{
				Repository:        "foo",
				TriggerTarget:     "pull_request",
				PullRequestNumber: pullReqNumber,
			},
			repo: fooRepo,
			pipelineRuns: []*pipelinev1.PipelineRun{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pr-foo-1",
						Namespace: "foo",
						Labels: map[string]string{
							keys.OriginalPRName:   "pr-foo",
							keys.URLRepository:    formatting.CleanValueKubernetes("foo"),
							keys.SHA:              formatting.CleanValueKubernetes("foosha"),
							keys.PullRequest:      strconv.Itoa(pullReqNumber),
							keys.EventType:        string(triggertype.PullRequest),
							keys.CancelInProgress: "false",
						},
					},
					Spec: pipelinev1.PipelineRunSpec{},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pr-foo-2",
						Namespace: "foo",
						Labels: map[string]string{
							keys.OriginalPRName:   "pr-foo",
							keys.URLRepository:    formatting.CleanValueKubernetes("foo"),
							keys.SHA:              formatting.CleanValueKubernetes("foosha"),
							keys.PullRequest:      strconv.Itoa(pullReqNumber),
							keys.EventType:        string(triggertype.PullRequest),
							keys.CancelInProgress: "false",
						},
					},
					Spec: pipelinev1.PipelineRunSpec{},
				},
			},
			cancelledPipelineRuns: map[string]bool{},
		},
		{
			name: "cancel all in progress PipelineRuns with no annotation",
			event: &info.Event{
				Repository:        "foo",
				TriggerTarget:     "pull_request",
				PullRequestNumber: pullReqNumber,
			},
			repo: fooRepo,
			pipelineRuns: []*pipelinev1.PipelineRun{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pr-foo-1",
						Namespace: "foo",
						Labels: map[string]string{
							keys.OriginalPRName: "pr-foo",
							keys.URLRepository:  formatting.CleanValueKubernetes("foo"),
							keys.SHA:            formatting.CleanValueKubernetes("foosha"),
							keys.PullRequest:    strconv.Itoa(pullReqNumber),
							keys.EventType:      string(triggertype.PullRequest),
						},
					},
					Spec: pipelinev1.PipelineRunSpec{},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pr-foo-2",
						Namespace: "foo",
						Labels: map[string]string{
							keys.OriginalPRName: "pr-foo",
							keys.URLRepository:  formatting.CleanValueKubernetes("foo"),
							keys.SHA:            formatting.CleanValueKubernetes("foosha"),
							keys.PullRequest:    strconv.Itoa(pullReqNumber),
							keys.EventType:      string(triggertype.PullRequest),
						},
					},
					Spec: pipelinev1.PipelineRunSpec{},
				},
			},
			cancelledPipelineRuns: map[string]bool{},
		},
		{
			name: "no PipelineRuns to cancel",
			event: &info.Event{
				Repository:        "foo",
				TriggerTarget:     "pull_request",
				PullRequestNumber: pullReqNumber,
			},
			repo:                  fooRepo,
			pipelineRuns:          []*pipelinev1.PipelineRun{},
			cancelledPipelineRuns: map[string]bool{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)

			tdata := testclient.Data{
				PipelineRuns: tt.pipelineRuns,
			}
			stdata, _ := testclient.SeedTestData(t, ctx, tdata)
			cs := &params.Run{
				Clients: clients.Clients{
					Log:    logger,
					Tekton: stdata.Pipeline,
					Kube:   stdata.Kube,
				},
			}
			pac := NewPacs(tt.event, nil, cs, &info.PacOpts{}, nil, logger, nil)
			err := pac.cancelAllInProgressBelongingToClosedPullRequest(ctx, tt.repo)
			assert.NilError(t, err)

			got, err := cs.Clients.Tekton.TektonV1().PipelineRuns("foo").List(ctx, metav1.ListOptions{})
			assert.NilError(t, err)

			for _, pr := range got.Items {
				if _, ok := tt.cancelledPipelineRuns[pr.Name]; ok {
					assert.Equal(t, string(pr.Spec.Status), pipelinev1.PipelineRunSpecStatusCancelledRunFinally)
				} else {
					assert.Assert(t, string(pr.Spec.Status) != pipelinev1.PipelineRunSpecStatusCancelledRunFinally)
				}
			}
		})
	}
}

func TestGetLabelSelector(t *testing.T) {
	tests := []struct {
		name      string
		labelsMap map[string]string
		want      string
	}{
		{
			name: "single label",
			labelsMap: map[string]string{
				"app": "nginx",
			},
			want: "app=nginx",
		},
		{
			name: "multiple labels",
			labelsMap: map[string]string{
				"app":     "nginx",
				"version": "1.14.2",
			},
			want: "app=nginx,version=1.14.2",
		},
		{
			name:      "empty labels",
			labelsMap: map[string]string{},
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getLabelSelector(tt.labelsMap); got != tt.want {
				t.Errorf("getLabelSelector() = %v, want %v", got, tt.want)
			}
		})
	}
}
