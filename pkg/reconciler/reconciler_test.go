package reconciler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-github/v47/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/consoleui"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/metrics"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	ghprovider "github.com/openshift-pipelines/pipelines-as-code/pkg/provider/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/sync"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	ghtesthelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test/github"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rtesting "knative.dev/pkg/reconciler/testing"
	"sigs.k8s.io/yaml"
)

var (
	finalSuccessStatus = "success"
	finalSuccessText   = `
<table>
  <tr><th>Status</th><th>Duration</th><th>Name</th></tr>
<tr>
<td>✅ Succeeded</td>
<td>42 seconds</td><td>

[fetch-repository](https://giphy.com/search/random-cats)

</td></tr>
<tr>
<td>✅ Succeeded</td>
<td>18 seconds</td><td>

[noop-task](https://giphy.com/search/random-cats)

</td></tr>
</table>`

	finalFailureStatus = "failure"
	finalFailureText   = `
<table>
  <tr><th>Status</th><th>Duration</th><th>Name</th></tr>
<tr>
<td>✅ Succeeded</td>
<td>42 seconds</td><td>

[fetch-repository](https://giphy.com/search/random-cats)

</td></tr>
<tr>
<td>❌ Failed</td>
<td>19 seconds</td><td>

[noop-task](https://giphy.com/search/random-cats)

</td></tr>
</table>`

	testRepo = &v1alpha1.Repository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sm43-pac-app",
			Namespace: "pac-app-pipelines",
		},
		Spec: v1alpha1.RepositorySpec{
			URL: "https://github.com/sm43/pac-app",
		},
	}
)

func TestReconciler_ReconcileKind(t *testing.T) {
	observer, _ := zapobserver.New(zap.InfoLevel)
	fakelogger := zap.New(observer).Sugar()
	fakeclient, mux, _, teardown := ghtesthelper.SetupGH()
	defer teardown()

	vcx := &ghprovider.Provider{
		Client: fakeclient,
		Token:  github.String("None"),
	}

	runEvent := info.Event{
		Organization: "sm43",
		Repository:   "pac-app",
	}

	tests := []struct {
		name            string
		pipelineRunfile string
		finalStatus     string
		finalStatusText string
		checkRunID      string
	}{
		{
			name:            "success pipelinerun",
			pipelineRunfile: "test-succeeded-pipelinerun",
			checkRunID:      "6566930541",
			finalStatus:     finalSuccessStatus,
			finalStatusText: finalSuccessText,
		},
		{
			name:            "failed pipelinerun",
			pipelineRunfile: "test-failed-pipelinerun",
			finalStatus:     finalFailureStatus,
			finalStatusText: finalFailureText,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			b, err := os.ReadFile(fmt.Sprintf("testdata/%s.yaml", tt.pipelineRunfile))
			if err != nil {
				t.Fatalf("ReadFile() = %v", err)
			}
			pr := v1beta1.PipelineRun{}
			if err := yaml.Unmarshal(b, &pr); err != nil {
				t.Fatalf("yaml.Unmarshal() = %v", err)
			}

			secretName := pr.Annotations[filepath.Join(pipelinesascode.GroupName, "git-auth-secret")]

			testData := testclient.Data{
				Repositories: []*v1alpha1.Repository{testRepo},
				PipelineRuns: []*v1beta1.PipelineRun{&pr},
				Secret: []*corev1.Secret{
					{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: testRepo.Namespace,
							Name:      secretName,
						},
					},
				},
			}
			stdata, _ := testclient.SeedTestData(t, ctx, testData)

			testSetupGHReplies(t, mux, runEvent, tt.checkRunID, tt.finalStatus, tt.finalStatusText)

			metrics, err := metrics.NewRecorder()
			assert.NilError(t, err)

			r := Reconciler{
				qm: sync.NewQueueManager(fakelogger),
				run: &params.Run{
					Clients: clients.Clients{
						PipelineAsCode: stdata.PipelineAsCode,
						Tekton:         stdata.Pipeline,
						Kube:           stdata.Kube,
						ConsoleUI:      consoleui.FallBackConsole{},
					},
					Info: info.Info{
						Pac: &info.PacOpts{
							SecretAutoCreation: true,
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

			event := buildEventFromPipelineRun(&pr)
			err = r.reportFinalStatus(ctx, fakelogger, event, &pr, vcx)
			assert.NilError(t, err)

			got, err := stdata.Pipeline.TektonV1beta1().PipelineRuns(pr.Namespace).Get(ctx, pr.Name, metav1.GetOptions{})
			assert.NilError(t, err)

			// make sure secret is deleted
			_, err = stdata.Kube.CoreV1().Secrets(testRepo.Namespace).Get(ctx, secretName, metav1.GetOptions{})
			assert.Error(t, err, fmt.Sprintf("secrets \"%s\" not found", secretName))

			// state must be updated to completed
			assert.Equal(t, got.Labels[filepath.Join(pipelinesascode.GroupName, "state")], kubeinteraction.StateCompleted)
		})
	}
}

func testSetupGHReplies(t *testing.T, mux *http.ServeMux, runevent info.Event, checkrunID, finalStatus, finalStatusText string) {
	mux.HandleFunc(fmt.Sprintf("/repos/%s/%s/check-runs/%s", runevent.Organization, runevent.Repository, checkrunID),
		func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			created := github.CreateCheckRunOptions{}
			err := json.Unmarshal(body, &created)
			assert.NilError(t, err)
			if created.GetStatus() == "completed" {
				assert.Equal(t, created.GetConclusion(), finalStatus, "we got the status `%s` but we should have get the status `%s`", created.GetConclusion(), finalStatus)
				assert.Assert(t, strings.Contains(created.GetOutput().GetText(), finalStatusText),
					"GetStatus/CheckRun %s != %s", created.GetOutput().GetText(), finalStatusText)
			}
		})
}

func TestUpdatePipelineRunState(t *testing.T) {
	observer, _ := zapobserver.New(zap.InfoLevel)
	fakelogger := zap.New(observer).Sugar()

	tests := []struct {
		name        string
		pipelineRun *v1beta1.PipelineRun
		state       string
	}{
		{
			name: "queued to started",
			pipelineRun: &v1beta1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "test",
					Labels: map[string]string{
						filepath.Join(pipelinesascode.GroupName, "state"): kubeinteraction.StateQueued,
					},
				},
				Spec: v1beta1.PipelineRunSpec{
					Status: v1beta1.PipelineRunSpecStatusPending,
				},
				Status: v1beta1.PipelineRunStatus{},
			},
			state: kubeinteraction.StateStarted,
		},
		{
			name: "started to completed",
			pipelineRun: &v1beta1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "test",
					Labels: map[string]string{
						filepath.Join(pipelinesascode.GroupName, "state"): kubeinteraction.StateStarted,
					},
				},
				Spec:   v1beta1.PipelineRunSpec{},
				Status: v1beta1.PipelineRunStatus{},
			},
			state: kubeinteraction.StateCompleted,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			testData := testclient.Data{
				PipelineRuns: []*v1beta1.PipelineRun{tt.pipelineRun},
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

			assert.Equal(t, updatedPR.Labels[filepath.Join(pipelinesascode.GroupName, "state")], tt.state)
			assert.Equal(t, updatedPR.Spec.Status, v1beta1.PipelineRunSpecStatus(""))
		})
	}
}
