package reconciler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/google/go-github/v59/github"
	"github.com/jonboulle/clockwork"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/consoleui"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/metrics"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
	ghprovider "github.com/openshift-pipelines/pipelines-as-code/pkg/provider/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/secrets"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/sync"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	ghtesthelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test/github"
	tektontest "github.com/openshift-pipelines/pipelines-as-code/pkg/test/tekton"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/golden"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	knativeapi "knative.dev/pkg/apis"
	knativeduckv1 "knative.dev/pkg/apis/duck/v1"
	rtesting "knative.dev/pkg/reconciler/testing"
)

var (
	randomURL          = "https://github.com/random/app"
	finalSuccessStatus = "success"
	finalFailureStatus = "failure"
)

func testSetupGHReplies(t *testing.T, mux *http.ServeMux, runevent *info.Event, checkrunID, finalStatus string) {
	t.Helper()
	mux.HandleFunc(fmt.Sprintf("/repos/%s/%s/check-runs/%s", runevent.Organization, runevent.Repository, checkrunID),
		func(_ http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			created := github.CreateCheckRunOptions{}
			err := json.Unmarshal(body, &created)
			assert.NilError(t, err)
			if created.GetStatus() == "completed" {
				assert.Equal(t, created.GetConclusion(), finalStatus, "we got the status `%s` but we should have get the status `%s`", created.GetConclusion(), finalStatus)
				golden.Assert(t, created.GetOutput().GetText(), strings.ReplaceAll(fmt.Sprintf("%s.golden", t.Name()), "/", "-"))
			}
		})
}

func TestReconciler_ReconcileKind(t *testing.T) {
	observer, _ := zapobserver.New(zap.InfoLevel)
	fakelogger := zap.New(observer).Sugar()
	fakeclient, mux, _, teardown := ghtesthelper.SetupGH()
	defer teardown()

	vcx := &ghprovider.Provider{
		Client: fakeclient,
		Token:  github.String("None"),
	}

	tests := []struct {
		name          string
		finalStatus   string
		checkRunID    string
		exitCode      int32
		taskCondition knativeduckv1.Conditions
	}{
		{
			name:        "success pipelinerun",
			checkRunID:  "6566930541",
			finalStatus: finalSuccessStatus,
			exitCode:    0,
			taskCondition: knativeduckv1.Conditions{
				{
					Type:   knativeapi.ConditionSucceeded,
					Status: corev1.ConditionTrue,
					Reason: string(tektonv1.PipelineRunReasonSuccessful),
				},
			},
		},
		{
			name:        "failed pipelinerun",
			finalStatus: finalFailureStatus,
			exitCode:    1,
			checkRunID:  "6566930542",
			taskCondition: knativeduckv1.Conditions{
				{
					Type:   knativeapi.ConditionSucceeded,
					Status: corev1.ConditionFalse,
					Reason: string(tektonv1.PipelineRunReasonFailed),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			clock := clockwork.NewFakeClock()
			statusPR := tektonv1.PipelineRunReasonSuccessful
			if tt.finalStatus == finalFailureStatus {
				statusPR = tektonv1.PipelineRunReasonFailed
			}
			pr := tektontest.MakePRCompletion(clock, "pipeline-newest", "ns", string(statusPR), nil, make(map[string]string), 10)
			pr.Status.ChildReferences = []tektonv1.ChildStatusReference{
				{
					TypeMeta: runtime.TypeMeta{
						Kind: "TaskRun",
					},
					Name:             "task1",
					PipelineTaskName: "task1",
				},
			}

			secretName := secrets.GenerateBasicAuthSecretName()
			ctx = info.StoreCurrentControllerName(ctx, "default")

			pr.Annotations = map[string]string{
				keys.GitAuthSecret:  secretName,
				keys.State:          kubeinteraction.StateCompleted,
				keys.InstallationID: "1234",
				keys.RepoURL:        randomURL,
				keys.Repository:     pr.GetName(),
				keys.OriginalPRName: pr.GetName(),
				keys.CheckRunID:     tt.checkRunID,
			}
			pr.Labels = map[string]string{
				keys.Repository: pr.GetName(),
			}

			testRepo := &v1alpha1.Repository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pr.GetName(),
					Namespace: pr.GetNamespace(),
				},
				Spec: v1alpha1.RepositorySpec{
					URL: randomURL,
				},
			}

			taskStatus := tektonv1.TaskRunStatusFields{
				PodName: "task1",
				Steps: []tektonv1.StepState{
					{
						Name: "step1",
						ContainerState: corev1.ContainerState{
							Terminated: &corev1.ContainerStateTerminated{
								ExitCode: tt.exitCode,
							},
						},
					},
				},
			}
			testData := testclient.Data{
				Repositories: []*v1alpha1.Repository{testRepo},
				PipelineRuns: []*tektonv1.PipelineRun{pr},
				TaskRuns: []*tektonv1.TaskRun{
					tektontest.MakeTaskRunCompletion(clock, "task1", "ns", "pipeline-newest",
						map[string]string{}, taskStatus, tt.taskCondition, 10),
				},
				Secret: []*corev1.Secret{
					{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: testRepo.Namespace,
							Name:      secretName,
						},
					},
				},
			}
			stdata, informers := testclient.SeedTestData(t, ctx, testData)

			metrics, err := metrics.NewRecorder()
			assert.NilError(t, err)

			r := Reconciler{
				repoLister: informers.Repository.Lister(),
				qm:         sync.NewQueueManager(fakelogger),
				run: &params.Run{
					Clients: clients.Clients{
						PipelineAsCode: stdata.PipelineAsCode,
						Tekton:         stdata.Pipeline,
						Kube:           stdata.Kube,
					},
					Info: info.Info{
						Pac: &info.PacOpts{
							Settings: settings.Settings{
								ErrorLogSnippet:    true,
								SecretAutoCreation: true,
							},
						},
						Controller: &info.ControllerInfo{
							Secret: secretName,
						},
					},
				},
				pipelineRunLister: stdata.PipelineLister,
				kinteract: &kubeinteraction.Interaction{
					Run: &params.Run{
						Clients: clients.Clients{
							Kube:   stdata.Kube,
							Tekton: stdata.Pipeline,
						},
					},
				},
				metrics: metrics,
			}
			r.run.Clients.SetConsoleUI(consoleui.FallBackConsole{})
			pacInfo := info.PacOpts{
				Settings: settings.Settings{
					ErrorLogSnippet:    true,
					SecretAutoCreation: true,
				},
			}
			vcx.SetPacInfo(&pacInfo)

			event := buildEventFromPipelineRun(pr)
			testSetupGHReplies(t, mux, event, tt.checkRunID, tt.finalStatus)

			_, err = r.reportFinalStatus(ctx, fakelogger, pacInfo, event, pr, vcx)
			assert.NilError(t, err)

			got, err := stdata.Pipeline.TektonV1().PipelineRuns(pr.Namespace).Get(ctx, pr.Name, metav1.GetOptions{})
			assert.NilError(t, err)

			// state must be updated to completed
			assert.Equal(t, got.Annotations[keys.State], kubeinteraction.StateCompleted)
		})
	}
}

func TestUpdatePipelineRunState(t *testing.T) {
	observer, _ := zapobserver.New(zap.InfoLevel)
	fakelogger := zap.New(observer).Sugar()

	tests := []struct {
		name        string
		pipelineRun *tektonv1.PipelineRun
		state       string
	}{
		{
			name: "queued to started",
			pipelineRun: &tektonv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "test",
					Annotations: map[string]string{
						keys.State: kubeinteraction.StateQueued,
					},
				},
				Spec: tektonv1.PipelineRunSpec{
					Status: tektonv1.PipelineRunSpecStatusPending,
				},
				Status: tektonv1.PipelineRunStatus{},
			},
			state: kubeinteraction.StateStarted,
		},
		{
			name: "started to completed",
			pipelineRun: &tektonv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "test",
					Annotations: map[string]string{
						keys.State: kubeinteraction.StateStarted,
					},
				},
				Spec:   tektonv1.PipelineRunSpec{},
				Status: tektonv1.PipelineRunStatus{},
			},
			state: kubeinteraction.StateCompleted,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			testData := testclient.Data{
				PipelineRuns: []*tektonv1.PipelineRun{tt.pipelineRun},
			}
			stdata, _ := testclient.SeedTestData(t, ctx, testData)
			r := &Reconciler{
				run: &params.Run{
					Clients: clients.Clients{
						Tekton: stdata.Pipeline,
					},
				},
			}

			updatedPR, err := r.updatePipelineRunState(ctx, fakelogger, tt.pipelineRun, tt.state)
			assert.NilError(t, err)

			assert.Equal(t, updatedPR.Annotations[keys.State], tt.state)
			assert.Equal(t, updatedPR.Spec.Status, tektonv1.PipelineRunSpecStatus(""))
		})
	}
}
