package pipelineascode

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/go-github/v74/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/consoleui"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/matcher"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	ghprovider "github.com/openshift-pipelines/pipelines-as-code/pkg/provider/github"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	ghtesthelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test/github"
	kitesthelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test/kubernetestint"
	testprovider "github.com/openshift-pipelines/pipelines-as-code/pkg/test/provider"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stesting "k8s.io/client-go/testing"
	knativeapis "knative.dev/pkg/apis"
	rtesting "knative.dev/pkg/reconciler/testing"
)

// KinterfaceTestWithSecretTracking tracks secret creation attempts and simulates AlreadyExists for concurrent access.
type KinterfaceTestWithSecretTracking struct {
	kitesthelper.KinterfaceTest
	// secretCreationCount uses atomic.Int32 to ensure thread-safe counting across concurrent goroutines
	// without requiring explicit locks. This is critical for accurately simulating race conditions in tests
	// where multiple goroutines may attempt to create secrets simultaneously.
	secretCreationCount atomic.Int32
}

func (k *KinterfaceTestWithSecretTracking) CreateSecret(ctx context.Context, ns string, secret *corev1.Secret) error {
	// Atomically increment and get the new count
	count := k.secretCreationCount.Add(1)

	// First creation succeeds, subsequent ones return AlreadyExists to simulate race condition
	if count > 1 {
		return errors.NewAlreadyExists(schema.GroupResource{Group: "", Resource: "secrets"}, secret.GetName())
	}

	return k.KinterfaceTest.CreateSecret(ctx, ns, secret)
}

func intPtr(i int) *int {
	return &i
}

// startPRTestConfig holds optional configuration for startPR test setup.
type startPRTestConfig struct {
	// KinterfaceTest fields
	consoleURL        string
	createSecretError error
	updateSecretError error
	deleteSecretError error

	// PacOpts fields
	secretAutoCreation bool

	// Provider fields
	createStatusErorring bool
}

// defaultStartPRTestConfig returns defaults for most tests.
func defaultStartPRTestConfig() startPRTestConfig {
	return startPRTestConfig{
		consoleURL:         "https://console.test",
		secretAutoCreation: true,
	}
}

// startPRTestFixture holds all the components needed for a startPR test.
type startPRTestFixture struct {
	cs         *params.Run
	event      *info.Event
	logger     *zap.SugaredLogger
	ctx        context.Context
	fakeclient *github.Client
	teardown   func()
	kint       *kitesthelper.KinterfaceTest
	pacInfo    *info.PacOpts
	vcx        provider.Interface
	pac        PacRun
}

func setupStartPRTestWithConfig(t *testing.T, config startPRTestConfig) *startPRTestFixture {
	t.Helper()
	cs, event, logger, ctx, fakeclient, teardown := setupStartPRTest(t)

	kint := &kitesthelper.KinterfaceTest{
		ConsoleURL:        config.consoleURL,
		CreateSecretError: config.createSecretError,
		UpdateSecretError: config.updateSecretError,
		DeleteSecretError: config.deleteSecretError,
	}

	pacInfo := &info.PacOpts{
		Settings: settings.Settings{
			SecretAutoCreation: config.secretAutoCreation,
		},
	}

	var vcx provider.Interface
	if config.createStatusErorring {
		vcx = &testprovider.TestProviderImp{
			CreateStatusErorring: true,
		}
		vcx.SetPacInfo(pacInfo)
	} else {
		vcx = setupProviderForTest(cs, logger, fakeclient, pacInfo)
	}

	return &startPRTestFixture{
		cs:         cs,
		event:      event,
		logger:     logger,
		ctx:        ctx,
		fakeclient: fakeclient,
		teardown:   teardown,
		kint:       kint,
		pacInfo:    pacInfo,
		vcx:        vcx,
		pac:        NewPacs(event, vcx, cs, pacInfo, kint, logger, nil),
	}
}

// setupStartPRTestDefault creates a test fixture with default configuration.
// Use this for most tests - it provides a complete, ready-to-use test environment.
func setupStartPRTestDefault(t *testing.T) *startPRTestFixture {
	t.Helper()
	return setupStartPRTestWithConfig(t, defaultStartPRTestConfig())
}

// setupStartPRTest creates common test fixtures for startPR tests.
func setupStartPRTest(t *testing.T) (*params.Run, *info.Event, *zap.SugaredLogger, context.Context, *github.Client, func()) {
	t.Helper()
	ctx, _ := rtesting.SetupFakeContext(t)
	fakeclient, mux, ghTestServerURL, teardown := ghtesthelper.SetupGH()
	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()

	stdata, _ := testclient.SeedTestData(t, ctx, testclient.Data{
		Namespaces: []*corev1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-namespace",
				},
			},
		},
	})

	cs := &params.Run{
		Clients: clients.Clients{
			PipelineAsCode: stdata.PipelineAsCode,
			Log:            logger,
			Kube:           stdata.Kube,
			Tekton:         stdata.Pipeline,
		},
		Info: info.Info{
			Controller: &info.ControllerInfo{
				Name:      "default",
				Configmap: "pipelines-as-code",
				Secret:    "pipelines-as-code-secret",
			},
		},
	}
	cs.Clients.SetConsoleUI(consoleui.FallBackConsole{})

	event := &info.Event{
		SHA:               "test-sha",
		Organization:      "test-org",
		Repository:        "test-repo",
		URL:               "https://test.com/repo",
		HeadBranch:        "test-branch",
		BaseBranch:        "main",
		Sender:            "test-user",
		EventType:         "pull_request",
		TriggerTarget:     "pull_request",
		PullRequestNumber: 123,
		Provider: &info.Provider{
			Token: "test-token",
			User:  "git",
			URL:   ghTestServerURL,
		},
	}

	// Setup common GitHub API mock responses
	replyString(mux, fmt.Sprintf("/repos/%s/%s/statuses/%s", event.Organization, event.Repository, event.SHA), "{}")
	replyString(mux, fmt.Sprintf("/repos/%s/%s/check-runs", event.Organization, event.Repository), `{"id": 123}`)

	return cs, event, logger, ctx, fakeclient, teardown
}

// setupProviderForTest creates and configures a GitHub provider for testing.
func setupProviderForTest(cs *params.Run, logger *zap.SugaredLogger, fakeclient *github.Client, pacInfo *info.PacOpts) *ghprovider.Provider {
	vcx := &ghprovider.Provider{
		Run:    cs,
		Token:  github.Ptr("test-token"),
		Logger: logger,
	}
	vcx.SetGithubClient(fakeclient)
	vcx.SetPacInfo(pacInfo)
	return vcx
}

// createTestMatch creates a Match object for testing startPR.
func createTestMatch(withSecret bool, concurrencyLimit *int) matcher.Match {
	namespace := "test-namespace"
	prName := "test-pr-"
	annotations := make(map[string]string)
	labels := make(map[string]string)

	if withSecret {
		annotations[keys.GitAuthSecret] = "test-git-secret"
	}

	pr := &pipelinev1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: prName,
			Namespace:    namespace,
			Annotations:  annotations,
			Labels:       labels,
		},
		Spec: pipelinev1.PipelineRunSpec{
			PipelineRef: &pipelinev1.PipelineRef{
				Name: "test-pipeline",
			},
		},
	}

	repo := &v1alpha1.Repository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-repo",
			Namespace: namespace,
		},
		Spec: v1alpha1.RepositorySpec{
			ConcurrencyLimit: concurrencyLimit,
		},
	}

	return matcher.Match{
		PipelineRun: pr,
		Repo:        repo,
		Config:      map[string]string{},
	}
}

func TestStartPR(t *testing.T) {
	tests := []struct {
		name string
		test func(t *testing.T)
	}{
		{
			name: "success - comprehensive success test covering the full startPR flow",
			test: func(t *testing.T) {
				fixture := setupStartPRTestDefault(t)
				defer fixture.teardown()

				match := createTestMatch(true, nil)

				pr, err := fixture.pac.startPR(fixture.ctx, match)

				assert.NilError(t, err)
				assert.Assert(t, pr != nil)
				assert.Equal(t, pr.GetNamespace(), "test-namespace")
				// GenerateName is used before K8s generates the actual name
				assert.Assert(t, pr.GetGenerateName() != "" || pr.GetName() != "", "PipelineRun should have a GenerateName or Name")

				_, hasState := pr.GetAnnotations()[keys.State]
				assert.Assert(t, hasState, "State annotation should be set")

				_, hasLogURL := pr.GetAnnotations()[keys.LogURL]
				assert.Assert(t, hasLogURL, "LogURL annotation should be set")
			},
		},
		{
			name: "missing secret annotation",
			test: func(t *testing.T) {
				fixture := setupStartPRTestDefault(t)
				defer fixture.teardown()

				match := createTestMatch(false, nil) // no secret annotation

				pr, err := fixture.pac.startPR(fixture.ctx, match)

				assert.Assert(t, pr == nil)
				assert.ErrorContains(t, err, "cannot get annotation")
				assert.ErrorContains(t, err, keys.GitAuthSecret)
			},
		},
		{
			name: "secret creation - secret already exists should succeed with warning",
			test: func(t *testing.T) {
				config := defaultStartPRTestConfig()
				config.createSecretError = errors.NewAlreadyExists(schema.GroupResource{Group: "", Resource: "secrets"}, "test-git-secret")
				config.secretAutoCreation = true
				fixture := setupStartPRTestWithConfig(t, config)
				defer fixture.teardown()

				match := createTestMatch(true, nil)
				pr, err := fixture.pac.startPR(fixture.ctx, match)

				assert.NilError(t, err)
				assert.Assert(t, pr != nil, "PipelineRun should be created")
				assert.Equal(t, pr.GetNamespace(), "test-namespace")
			},
		},
		{
			name: "secret creation - secret creation failure should fail",
			test: func(t *testing.T) {
				config := defaultStartPRTestConfig()
				config.createSecretError = fmt.Errorf("connection timeout")
				config.secretAutoCreation = true
				fixture := setupStartPRTestWithConfig(t, config)
				defer fixture.teardown()

				match := createTestMatch(true, nil)
				pr, err := fixture.pac.startPR(fixture.ctx, match)

				assert.Assert(t, pr == nil, "PipelineRun should be nil on error")
				assert.Assert(t, err != nil, "Error should be returned")
				assert.ErrorContains(t, err, "creating basic auth secret")
				assert.ErrorContains(t, err, "has failed")
			},
		},
		{
			name: "secret creation - auto-creation disabled should succeed without creating secret",
			test: func(t *testing.T) {
				config := defaultStartPRTestConfig()
				config.secretAutoCreation = false
				fixture := setupStartPRTestWithConfig(t, config)
				defer fixture.teardown()

				match := createTestMatch(true, nil)
				pr, err := fixture.pac.startPR(fixture.ctx, match)

				assert.NilError(t, err)
				assert.Assert(t, pr != nil, "PipelineRun should be created")
				assert.Equal(t, pr.GetNamespace(), "test-namespace")
			},
		},
		{
			name: "annotations - CancelInProgress annotation propagated to label",
			test: func(t *testing.T) {
				fixture := setupStartPRTestDefault(t)
				defer fixture.teardown()

				match := createTestMatch(true, nil)
				match.PipelineRun.Annotations[keys.CancelInProgress] = "true"

				pr, err := fixture.pac.startPR(fixture.ctx, match)

				assert.NilError(t, err)
				assert.Assert(t, pr != nil)
				value, ok := pr.GetLabels()[keys.CancelInProgress]
				assert.Assert(t, ok, "CancelInProgress label should be set")
				assert.Equal(t, value, "true")
			},
		},
		{
			name: "annotations - GitLab project IDs set as annotations",
			test: func(t *testing.T) {
				fixture := setupStartPRTestDefault(t)
				defer fixture.teardown()

				match := createTestMatch(true, nil)
				fixture.event.SourceProjectID = 123
				fixture.event.TargetProjectID = 456
				// Recreate the pac instance with updated event
				fixture.pac = NewPacs(fixture.event, fixture.vcx, fixture.cs, fixture.pacInfo, fixture.kint, fixture.logger, nil)

				pr, err := fixture.pac.startPR(fixture.ctx, match)

				assert.NilError(t, err)
				assert.Assert(t, pr != nil)
				sourceID, ok := pr.GetAnnotations()[keys.SourceProjectID]
				assert.Assert(t, ok, "SourceProjectID annotation should be set")
				assert.Equal(t, sourceID, "123")
				targetID, ok := pr.GetAnnotations()[keys.TargetProjectID]
				assert.Assert(t, ok, "TargetProjectID annotation should be set")
				assert.Equal(t, targetID, "456")
			},
		},
		{
			name: "annotations - OriginalPRName annotation preserved",
			test: func(t *testing.T) {
				fixture := setupStartPRTestDefault(t)
				defer fixture.teardown()

				match := createTestMatch(true, nil)
				match.PipelineRun.Annotations[keys.OriginalPRName] = "original-pr-name"

				pr, err := fixture.pac.startPR(fixture.ctx, match)

				assert.NilError(t, err)
				assert.Assert(t, pr != nil)
				value, ok := pr.GetAnnotations()[keys.OriginalPRName]
				assert.Assert(t, ok, "OriginalPRName annotation should be set")
				assert.Equal(t, value, "original-pr-name")
			},
		},
		{
			name: "concurrency limit - nil concurrency limit starts immediately",
			test: func(t *testing.T) {
				fixture := setupStartPRTestDefault(t)
				defer fixture.teardown()

				match := createTestMatch(true, nil)
				pr, err := fixture.pac.startPR(fixture.ctx, match)

				assert.NilError(t, err)
				assert.Assert(t, pr != nil)
				state, hasState := pr.GetAnnotations()[keys.State]
				assert.Assert(t, hasState, "State annotation should be set")
				assert.Equal(t, state, kubeinteraction.StateStarted)
				assert.Equal(t, pr.Spec.Status, pipelinev1.PipelineRunSpecStatus(""))
				_, hasStarted := pr.GetAnnotations()[keys.SCMReportingPLRStarted]
				assert.Assert(t, hasStarted, "SCMReportingPLRStarted should be set for started PRs")
			},
		},
		{
			name: "concurrency limit - zero concurrency limit treated as no limit",
			test: func(t *testing.T) {
				fixture := setupStartPRTestDefault(t)
				defer fixture.teardown()

				match := createTestMatch(true, intPtr(0))
				pr, err := fixture.pac.startPR(fixture.ctx, match)

				assert.NilError(t, err)
				assert.Assert(t, pr != nil)
				state := pr.GetAnnotations()[keys.State]
				assert.Equal(t, state, kubeinteraction.StateStarted)
			},
		},
		{
			name: "concurrency limit - positive concurrency limit sets pending",
			test: func(t *testing.T) {
				fixture := setupStartPRTestDefault(t)
				defer fixture.teardown()

				match := createTestMatch(true, intPtr(1))
				pr, err := fixture.pac.startPR(fixture.ctx, match)

				assert.NilError(t, err)
				assert.Assert(t, pr != nil)
				state := pr.GetAnnotations()[keys.State]
				assert.Equal(t, state, kubeinteraction.StateQueued)
				assert.Equal(t, string(pr.Spec.Status), string(pipelinev1.PipelineRunSpecStatusPending))
				_, hasStarted := pr.GetAnnotations()[keys.SCMReportingPLRStarted]
				assert.Assert(t, !hasStarted, "SCMReportingPLRStarted should NOT be set for queued PRs")
			},
		},
		{
			name: "concurrency limit - higher concurrency limit still sets pending",
			test: func(t *testing.T) {
				fixture := setupStartPRTestDefault(t)
				defer fixture.teardown()

				match := createTestMatch(true, intPtr(5))
				pr, err := fixture.pac.startPR(fixture.ctx, match)

				assert.NilError(t, err)
				assert.Assert(t, pr != nil)
				state := pr.GetAnnotations()[keys.State]
				assert.Equal(t, state, kubeinteraction.StateQueued)
			},
		},
		{
			name: "concurrency limit - negative concurrency limit treated as having limit (queued)",
			test: func(t *testing.T) {
				fixture := setupStartPRTestDefault(t)
				defer fixture.teardown()

				match := createTestMatch(true, intPtr(-1))
				pr, err := fixture.pac.startPR(fixture.ctx, match)

				assert.NilError(t, err)
				assert.Assert(t, pr != nil)
				state := pr.GetAnnotations()[keys.State]
				assert.Equal(t, state, kubeinteraction.StateQueued)
			},
		},
		{
			name: "creation failure - with secret cleanup",
			test: func(t *testing.T) {
				ctx, _ := rtesting.SetupFakeContext(t)
				observer, _ := zapobserver.New(zap.InfoLevel)
				logger := zap.New(observer).Sugar()

				stdata, _ := testclient.SeedTestData(t, ctx, testclient.Data{
					Namespaces: []*corev1.Namespace{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-namespace",
							},
						},
					},
				})

				creationFailed := false
				stdata.Pipeline.PrependReactor("create", "pipelineruns", func(_ k8stesting.Action) (handled bool, ret runtime.Object, err error) {
					if !creationFailed {
						creationFailed = true
						return true, nil, fmt.Errorf("namespace quota exceeded")
					}
					return false, nil, nil
				})

				cs := &params.Run{
					Clients: clients.Clients{
						PipelineAsCode: stdata.PipelineAsCode,
						Log:            logger,
						Kube:           stdata.Kube,
						Tekton:         stdata.Pipeline,
					},
					Info: info.Info{
						Controller: &info.ControllerInfo{
							Name:      "default",
							Configmap: "pipelines-as-code",
							Secret:    "pipelines-as-code-secret",
						},
					},
				}
				cs.Clients.SetConsoleUI(consoleui.FallBackConsole{})

				event := &info.Event{
					SHA:               "test-sha",
					Organization:      "test-org",
					Repository:        "test-repo",
					URL:               "https://test.com/repo",
					HeadBranch:        "test-branch",
					BaseBranch:        "main",
					Sender:            "test-user",
					EventType:         "pull_request",
					TriggerTarget:     "pull_request",
					PullRequestNumber: 123,
					Provider: &info.Provider{
						Token: "test-token",
						User:  "git",
						URL:   "https://api.github.com",
					},
				}

				match := createTestMatch(true, nil)
				kint := &kitesthelper.KinterfaceTest{
					ConsoleURL: "https://console.test",
				}
				pacInfo := &info.PacOpts{
					Settings: settings.Settings{
						SecretAutoCreation: true,
					},
				}

				fakeclient, _, _, teardown := ghtesthelper.SetupGH()
				defer teardown()

				vcx := setupProviderForTest(cs, logger, fakeclient, pacInfo)
				p := NewPacs(event, vcx, cs, pacInfo, kint, logger, nil)

				pr, err := p.startPR(ctx, match)

				assert.Assert(t, pr == nil, "PipelineRun should be nil on creation failure")
				assert.ErrorContains(t, err, "creating pipelinerun")
				assert.ErrorContains(t, err, "has failed")
				assert.Assert(t, kint.SecretDeleted, "Secret should have been deleted on PR creation failure")
			},
		},
		{
			name: "creation failure - secret cleanup error",
			test: func(t *testing.T) {
				ctx, _ := rtesting.SetupFakeContext(t)
				observer, log := zapobserver.New(zap.InfoLevel)
				logger := zap.New(observer).Sugar()

				stdata, _ := testclient.SeedTestData(t, ctx, testclient.Data{
					Namespaces: []*corev1.Namespace{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-namespace",
							},
						},
					},
				})

				stdata.Pipeline.PrependReactor("create", "pipelineruns", func(_ k8stesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, fmt.Errorf("api server unavailable")
				})

				cs := &params.Run{
					Clients: clients.Clients{
						PipelineAsCode: stdata.PipelineAsCode,
						Log:            logger,
						Kube:           stdata.Kube,
						Tekton:         stdata.Pipeline,
					},
					Info: info.Info{
						Controller: &info.ControllerInfo{
							Name:      "default",
							Configmap: "pipelines-as-code",
							Secret:    "pipelines-as-code-secret",
						},
					},
				}
				cs.Clients.SetConsoleUI(consoleui.FallBackConsole{})

				event := &info.Event{
					SHA:               "test-sha",
					Organization:      "test-org",
					Repository:        "test-repo",
					URL:               "https://test.com/repo",
					HeadBranch:        "test-branch",
					BaseBranch:        "main",
					Sender:            "test-user",
					EventType:         "pull_request",
					TriggerTarget:     "pull_request",
					PullRequestNumber: 123,
					Provider: &info.Provider{
						Token: "test-token",
						User:  "git",
						URL:   "https://api.github.com",
					},
				}

				match := createTestMatch(true, nil)
				kint := &kitesthelper.KinterfaceTest{
					ConsoleURL:        "https://console.test",
					DeleteSecretError: fmt.Errorf("failed to delete secret"),
				}
				pacInfo := &info.PacOpts{
					Settings: settings.Settings{
						SecretAutoCreation: true,
					},
				}

				fakeclient, _, _, teardown := ghtesthelper.SetupGH()
				defer teardown()

				vcx := setupProviderForTest(cs, logger, fakeclient, pacInfo)
				p := NewPacs(event, vcx, cs, pacInfo, kint, logger, nil)

				pr, err := p.startPR(ctx, match)

				assert.Assert(t, pr == nil, "PipelineRun should be nil")
				assert.ErrorContains(t, err, "creating pipelinerun")
				assert.ErrorContains(t, err, "has failed")
				assert.Assert(t, kint.SecretDeleted, "Secret deletion should have been attempted")

				logEntries := log.FilterMessageSnippet("removing auto created secret").TakeAll()
				assert.Assert(t, len(logEntries) > 0, "Should have logged secret deletion error")
			},
		},
		{
			name: "ownerRef update - generic update failure should return PR with error",
			test: func(t *testing.T) {
				config := defaultStartPRTestConfig()
				config.updateSecretError = fmt.Errorf("failed to update ownerRef")
				fixture := setupStartPRTestWithConfig(t, config)
				defer fixture.teardown()

				match := createTestMatch(true, nil)
				pr, err := fixture.pac.startPR(fixture.ctx, match)

				assert.Assert(t, pr != nil, "PipelineRun should still be returned despite ownerRef update failure")
				assert.Assert(t, err != nil, "Error should be returned")
				assert.ErrorContains(t, err, "cannot update pipelinerun")
				assert.ErrorContains(t, err, "with ownerRef")
				assert.Equal(t, pr.GetNamespace(), "test-namespace")
				assert.Assert(t, pr.GetName() != "" || pr.GetGenerateName() != "", "PR should have a name")
			},
		},
		{
			name: "ownerRef update - conflict error should return PR with error",
			test: func(t *testing.T) {
				config := defaultStartPRTestConfig()
				config.updateSecretError = errors.NewConflict(schema.GroupResource{Group: "", Resource: "secrets"}, "test-git-secret", fmt.Errorf("resource version mismatch"))
				fixture := setupStartPRTestWithConfig(t, config)
				defer fixture.teardown()

				match := createTestMatch(true, nil)
				pr, err := fixture.pac.startPR(fixture.ctx, match)

				assert.Assert(t, pr != nil, "PipelineRun should still be returned despite ownerRef update failure")
				assert.Assert(t, err != nil, "Error should be returned")
				assert.ErrorContains(t, err, "cannot update pipelinerun")
				assert.ErrorContains(t, err, "with ownerRef")
			},
		},
		{
			name: "state management - no concurrency starts immediately",
			test: func(t *testing.T) {
				fixture := setupStartPRTestDefault(t)
				defer fixture.teardown()

				match := createTestMatch(true, nil)
				pr, err := fixture.pac.startPR(fixture.ctx, match)

				assert.NilError(t, err)
				assert.Assert(t, pr != nil)
				state, hasState := pr.GetAnnotations()[keys.State]
				assert.Assert(t, hasState, "State annotation should be set")
				assert.Equal(t, state, kubeinteraction.StateStarted)
				assert.Equal(t, string(pr.Spec.Status), "")
				scmStarted, hasStarted := pr.GetAnnotations()[keys.SCMReportingPLRStarted]
				assert.Assert(t, hasStarted, "SCMReportingPLRStarted should be set for started PRs")
				assert.Equal(t, scmStarted, "true")
			},
		},
		{
			name: "state management - with concurrency limit queued state",
			test: func(t *testing.T) {
				fixture := setupStartPRTestDefault(t)
				defer fixture.teardown()

				match := createTestMatch(true, intPtr(2))
				pr, err := fixture.pac.startPR(fixture.ctx, match)

				assert.NilError(t, err)
				assert.Assert(t, pr != nil)
				state := pr.GetAnnotations()[keys.State]
				assert.Equal(t, state, kubeinteraction.StateQueued)
				assert.Equal(t, string(pr.Spec.Status), string(pipelinev1.PipelineRunSpecStatusPending))
				_, hasStarted := pr.GetAnnotations()[keys.SCMReportingPLRStarted]
				assert.Assert(t, !hasStarted, "SCMReportingPLRStarted should NOT be set for queued PRs")
			},
		},
		{
			name: "status creation failure",
			test: func(t *testing.T) {
				config := defaultStartPRTestConfig()
				config.createStatusErorring = true
				fixture := setupStartPRTestWithConfig(t, config)
				defer fixture.teardown()

				match := createTestMatch(true, nil)
				pr, err := fixture.pac.startPR(fixture.ctx, match)

				assert.Assert(t, pr != nil, "PipelineRun should be returned even when CreateStatus fails")
				assert.ErrorContains(t, err, "cannot use the API on the provider platform")
				assert.ErrorContains(t, err, "in_progress status")
			},
		},
		{
			name: "GitHub App - logURL handling",
			test: func(t *testing.T) {
				fixture := setupStartPRTestDefault(t)
				defer fixture.teardown()

				match := createTestMatch(true, nil)
				match.PipelineRun.Annotations[keys.InstallationID] = "12345"

				pr, err := fixture.pac.startPR(fixture.ctx, match)

				assert.NilError(t, err)
				assert.Assert(t, pr != nil)
				installationID, hasInstallationID := pr.GetAnnotations()[keys.InstallationID]
				assert.Assert(t, hasInstallationID, "InstallationID annotation should be present")
				assert.Equal(t, installationID, "12345")
				state, hasState := pr.GetAnnotations()[keys.State]
				assert.Assert(t, hasState, "State annotation should be set")
				assert.Assert(t, state == kubeinteraction.StateStarted || state == kubeinteraction.StateQueued)
			},
		},
		{
			name: "patch behavior - successful patch with all annotations set",
			test: func(t *testing.T) {
				fixture := setupStartPRTestDefault(t)
				defer fixture.teardown()

				match := createTestMatch(true, nil)
				pr, err := fixture.pac.startPR(fixture.ctx, match)

				assert.NilError(t, err)
				assert.Assert(t, pr != nil, "PipelineRun should be returned")
				state, hasState := pr.GetAnnotations()[keys.State]
				assert.Assert(t, hasState, "State annotation should be patched")
				assert.Assert(t, state == kubeinteraction.StateStarted || state == kubeinteraction.StateQueued,
					"State should be either started or queued")
				logURL, hasLogURL := pr.GetAnnotations()[keys.LogURL]
				assert.Assert(t, hasLogURL, "LogURL annotation should be patched")
				assert.Assert(t, logURL != "", "LogURL should not be empty")
			},
		},
		{
			name: "patch behavior - patch failure PR still returned with error",
			test: func(t *testing.T) {
				ctx, _ := rtesting.SetupFakeContext(t)
				observer, _ := zapobserver.New(zap.InfoLevel)
				logger := zap.New(observer).Sugar()

				stdata, _ := testclient.SeedTestData(t, ctx, testclient.Data{
					Namespaces: []*corev1.Namespace{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-namespace",
							},
						},
					},
				})

				patchAttempts := 0
				stdata.Pipeline.PrependReactor("patch", "pipelineruns", func(_ k8stesting.Action) (handled bool, ret runtime.Object, err error) {
					patchAttempts++
					if patchAttempts == 1 {
						return true, nil, fmt.Errorf("etcd unavailable")
					}
					return false, nil, nil
				})

				cs := &params.Run{
					Clients: clients.Clients{
						PipelineAsCode: stdata.PipelineAsCode,
						Log:            logger,
						Kube:           stdata.Kube,
						Tekton:         stdata.Pipeline,
					},
					Info: info.Info{
						Controller: &info.ControllerInfo{
							Name:      "default",
							Configmap: "pipelines-as-code",
							Secret:    "pipelines-as-code-secret",
						},
					},
				}
				cs.Clients.SetConsoleUI(consoleui.FallBackConsole{})

				event := &info.Event{
					SHA:               "test-sha",
					Organization:      "test-org",
					Repository:        "test-repo",
					URL:               "https://test.com/repo",
					HeadBranch:        "test-branch",
					BaseBranch:        "main",
					Sender:            "test-user",
					EventType:         "pull_request",
					TriggerTarget:     "pull_request",
					PullRequestNumber: 123,
					Provider: &info.Provider{
						Token: "test-token",
						User:  "git",
						URL:   "https://api.github.com",
					},
				}

				fakeclient, mux, ghTestServerURL, teardown := ghtesthelper.SetupGH()
				defer teardown()

				replyString(mux, fmt.Sprintf("/repos/%s/%s/statuses/%s", event.Organization, event.Repository, event.SHA), "{}")
				replyString(mux, fmt.Sprintf("/repos/%s/%s/check-runs", event.Organization, event.Repository), `{"id": 123}`)
				event.Provider.URL = ghTestServerURL

				kint := &kitesthelper.KinterfaceTest{
					ConsoleURL: "https://console.test",
				}
				pacInfo := &info.PacOpts{
					Settings: settings.Settings{
						SecretAutoCreation: true,
					},
				}

				vcx := setupProviderForTest(cs, logger, fakeclient, pacInfo)
				p := NewPacs(event, vcx, cs, pacInfo, kint, logger, nil)

				match := createTestMatch(true, nil)
				pr, err := p.startPR(ctx, match)

				assert.Assert(t, pr != nil, "PipelineRun should be returned even when patch fails")
				assert.Assert(t, err != nil, "Error should be returned")
				assert.ErrorContains(t, err, "cannot patch pipelinerun")
				assert.ErrorContains(t, err, "etcd unavailable")
				assert.Equal(t, pr.GetNamespace(), "test-namespace")
				assert.Assert(t, patchAttempts >= 1, "Patch should have been attempted")
			},
		},
		{
			name: "logging with conditions",
			test: func(t *testing.T) {
				observer, log := zapobserver.New(zap.InfoLevel)
				logger := zap.New(observer).Sugar()

				cs, event, _, ctx, fakeclient, teardown := setupStartPRTest(t)
				defer teardown()
				cs.Clients.Log = logger

				match := createTestMatch(true, nil)
				match.PipelineRun.Status.Conditions = []knativeapis.Condition{
					{
						Type:   "Succeeded",
						Status: "Unknown",
						Reason: "PipelineRunPending",
					},
				}

				kint := &kitesthelper.KinterfaceTest{
					ConsoleURL: "https://console.test",
				}
				pacInfo := &info.PacOpts{
					Settings: settings.Settings{
						SecretAutoCreation: true,
					},
				}

				vcx := setupProviderForTest(cs, logger, fakeclient, pacInfo)
				p := NewPacs(event, vcx, cs, pacInfo, kint, logger, nil)

				pr, err := p.startPR(ctx, match)

				assert.NilError(t, err)
				assert.Assert(t, pr != nil)

				logEntries := log.FilterMessageSnippet("patched successfully").TakeAll()
				assert.Assert(t, len(logEntries) > 0, "Should have log entry about successful patching")

				foundDetailedLog := false
				for _, entry := range logEntries {
					if strings.Contains(entry.Message, "Status reason:") {
						foundDetailedLog = true
						break
					}
				}
				assert.Assert(t, foundDetailedLog, "Should have detailed log with Status reason field")
			},
		},
		{
			name: "multiple custom annotations and labels",
			test: func(t *testing.T) {
				fixture := setupStartPRTestDefault(t)
				defer fixture.teardown()

				match := createTestMatch(true, nil)
				match.PipelineRun.Annotations[keys.GitAuthSecret] = "test-git-secret"
				match.PipelineRun.Annotations["custom.annotation.1"] = "value1"
				match.PipelineRun.Annotations["custom.annotation.2"] = "value2"
				match.PipelineRun.Annotations["custom.annotation.3"] = "value3"
				match.PipelineRun.Annotations["custom.annotation.4"] = "value4"
				match.PipelineRun.Annotations["custom.annotation.5"] = "value5"
				match.PipelineRun.Annotations[keys.CancelInProgress] = "true"
				match.PipelineRun.Annotations[keys.MaxKeepRuns] = "5"

				if match.PipelineRun.Labels == nil {
					match.PipelineRun.Labels = make(map[string]string)
				}
				match.PipelineRun.Labels["custom.label.1"] = "labelvalue1"
				match.PipelineRun.Labels["custom.label.2"] = "labelvalue2"
				match.PipelineRun.Labels["custom.label.3"] = "labelvalue3"
				match.PipelineRun.Labels["app"] = "test-app"
				match.PipelineRun.Labels["environment"] = "test"

				pr, err := fixture.pac.startPR(fixture.ctx, match)

				assert.NilError(t, err)
				assert.Assert(t, pr != nil)
				assert.Equal(t, pr.GetAnnotations()["custom.annotation.1"], "value1")
				assert.Equal(t, pr.GetAnnotations()["custom.annotation.2"], "value2")
				assert.Equal(t, pr.GetAnnotations()["custom.annotation.3"], "value3")
				assert.Equal(t, pr.GetAnnotations()["custom.annotation.4"], "value4")
				assert.Equal(t, pr.GetAnnotations()["custom.annotation.5"], "value5")
				assert.Equal(t, pr.GetLabels()["custom.label.1"], "labelvalue1")
				assert.Equal(t, pr.GetLabels()["custom.label.2"], "labelvalue2")
				assert.Equal(t, pr.GetLabels()["custom.label.3"], "labelvalue3")
				assert.Equal(t, pr.GetLabels()["app"], "test-app")
				assert.Equal(t, pr.GetLabels()["environment"], "test")
				_, hasState := pr.GetAnnotations()[keys.State]
				assert.Assert(t, hasState, "State annotation should be set")
				_, hasLogURL := pr.GetAnnotations()[keys.LogURL]
				assert.Assert(t, hasLogURL, "LogURL annotation should be set")
				cancelInProgress, ok := pr.GetLabels()[keys.CancelInProgress]
				assert.Assert(t, ok, "CancelInProgress label should be set")
				assert.Equal(t, cancelInProgress, "true")
			},
		},
		{
			name: "concurrent creation - multiple isolated PipelineRuns",
			test: func(t *testing.T) {
				cs, event, logger, ctx, fakeclient, teardown := setupStartPRTest(t)
				defer teardown()

				kint := &kitesthelper.KinterfaceTest{
					ConsoleURL: "https://console.test",
				}
				pacInfo := &info.PacOpts{
					Settings: settings.Settings{
						SecretAutoCreation: true,
					},
				}

				vcx := setupProviderForTest(cs, logger, fakeclient, pacInfo)

				numConcurrent := 5
				matches := make([]matcher.Match, numConcurrent)
				for i := range numConcurrent {
					matches[i] = createTestMatch(true, nil)
					matches[i].PipelineRun.Name = fmt.Sprintf("test-pr-%d", i)
					matches[i].PipelineRun.GenerateName = ""
					matches[i].PipelineRun.Annotations[keys.GitAuthSecret] = fmt.Sprintf("test-git-secret-%d", i)
				}

				results := runConcurrentStartPR(t, numConcurrent, func(idx int) (*pipelinev1.PipelineRun, error) {
					p := NewPacs(event, vcx, cs, pacInfo, kint, logger, nil)
					return p.startPR(ctx, matches[idx])
				})

				successCount := 0
				failureCount := 0
				for range numConcurrent {
					res := <-results
					if res.err == nil && res.pr != nil {
						successCount++
						assert.Assert(t, res.pr.GetName() != "", "PipelineRun should have a name")
						assert.Equal(t, res.pr.GetNamespace(), "test-namespace")
						_, hasState := res.pr.GetAnnotations()[keys.State]
						assert.Assert(t, hasState, fmt.Sprintf("State annotation should be set for PR %d", res.idx))
						secretName := res.pr.GetAnnotations()[keys.GitAuthSecret]
						assert.Equal(t, secretName, fmt.Sprintf("test-git-secret-%d", res.idx))
					} else if res.err != nil {
						failureCount++
						t.Logf("PipelineRun %d creation failed: %v", res.idx, res.err)
					}
				}

				assert.Equal(t, successCount, numConcurrent, "All concurrent PipelineRuns should succeed with proper isolation, got %d/%d (failures: %d)", successCount, numConcurrent, failureCount)
				t.Logf("Successfully created %d/%d concurrent PipelineRuns", successCount, numConcurrent)
			},
		},
		{
			name: "concurrent creation - with same secret name",
			test: func(t *testing.T) {
				cs, event, logger, ctx, fakeclient, teardown := setupStartPRTest(t)
				defer teardown()

				kintWithTracking := &KinterfaceTestWithSecretTracking{
					KinterfaceTest: kitesthelper.KinterfaceTest{
						ConsoleURL: "https://console.test",
					},
				}
				pacInfo := &info.PacOpts{
					Settings: settings.Settings{
						SecretAutoCreation: true,
					},
				}

				vcx := setupProviderForTest(cs, logger, fakeclient, pacInfo)

				numConcurrent := 3
				matches := make([]matcher.Match, numConcurrent)
				for i := range numConcurrent {
					matches[i] = createTestMatch(true, nil)
					matches[i].PipelineRun.Annotations[keys.GitAuthSecret] = "shared-git-secret"
					matches[i].PipelineRun.Name = fmt.Sprintf("test-pr-shared-%d", i)
					matches[i].PipelineRun.GenerateName = ""
				}

				results := runConcurrentStartPR(t, numConcurrent, func(idx int) (*pipelinev1.PipelineRun, error) {
					p := NewPacs(event, vcx, cs, pacInfo, kintWithTracking, logger, nil)
					return p.startPR(ctx, matches[idx])
				})

				successCount := 0
				failureCount := 0
				for range numConcurrent {
					res := <-results
					if res.err == nil && res.pr != nil {
						successCount++
						assert.Equal(t, res.pr.GetNamespace(), "test-namespace")
						secretName, ok := res.pr.GetAnnotations()[keys.GitAuthSecret]
						assert.Assert(t, ok, "GitAuthSecret annotation should be present")
						assert.Equal(t, secretName, "shared-git-secret")
						_, hasState := res.pr.GetAnnotations()[keys.State]
						assert.Assert(t, hasState, fmt.Sprintf("State should be set for PR %d", res.idx))
					} else {
						failureCount++
						t.Logf("PipelineRun %d failed: %v", res.idx, res.err)
					}
				}

				assert.Equal(t, successCount, numConcurrent, "All concurrent PipelineRuns should succeed with AlreadyExists handling, got %d/%d (failures: %d)", successCount, numConcurrent, failureCount)
				attempts := kintWithTracking.secretCreationCount.Load()
				assert.Assert(t, attempts >= 1, "Secret creation should have been attempted at least once, got %d attempts", attempts)
				t.Logf("Successfully created %d/%d concurrent PipelineRuns with shared secret (%d creation attempts)", successCount, numConcurrent, attempts)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.test)
	}
}

// startPRResult holds the result of a concurrent startPR call.
type startPRResult struct {
	pr  *pipelinev1.PipelineRun
	err error
	idx int
}

// runConcurrentStartPR executes multiple startPR calls concurrently using a synchronization barrier
// to ensure they all start at the same time. It returns a channel of results.
func runConcurrentStartPR(t *testing.T, count int, fn func(idx int) (*pipelinev1.PipelineRun, error)) chan startPRResult {
	t.Helper()

	results := make(chan startPRResult, count)
	ready := make(chan struct{}, count)
	start := make(chan struct{})

	for i := range count {
		go func(idx int) {
			// Signal ready and wait for start signal to ensure concurrent execution
			ready <- struct{}{}
			<-start

			pr, err := fn(idx)
			results <- startPRResult{pr: pr, err: err, idx: idx}
		}(i)
	}

	// Wait for all goroutines to be ready
	for range count {
		<-ready
	}

	// Start all goroutines simultaneously
	close(start)

	return results
}
