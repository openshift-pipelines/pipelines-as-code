package queue

import (
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"knative.dev/pkg/apis"

	"github.com/jonboulle/clockwork"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func skipOnOSX64(t *testing.T) {
	if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
		t.Skip("Skipping test on OSX arm64")
	}
}

func TestSomeoneElseSetPendingWithNoConcurrencyLimit(t *testing.T) {
	// Skip if we are running on OSX, there is a problem with ordering only happening on arm64
	skipOnOSX64(t)

	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()

	qm := NewManager(logger)
	repo := newTestRepo(1)
	// unset concurrency limit
	repo.Spec.ConcurrencyLimit = nil

	pr := newTestPR("first", time.Now(), nil, nil, tektonv1.PipelineRunSpec{})
	// set to pending
	pr.Status.Conditions = duckv1.Conditions{
		{
			Type:   apis.ConditionSucceeded,
			Reason: v1beta1.PipelineRunReasonPending.String(),
		},
	}
	started, err := qm.AddListToRunningQueue(repo, []string{PrKey(pr)})
	assert.NilError(t, err)
	assert.Equal(t, len(started), 1)
}

func TestAddToPendingQueueDirectly(t *testing.T) {
	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()

	qm := NewManager(logger)
	repo := newTestRepo(1)
	// unset concurrency limit
	repo.Spec.ConcurrencyLimit = nil

	pr := newTestPR("first", time.Now(), nil, nil, tektonv1.PipelineRunSpec{})
	// set to pending
	pr.Status.Conditions = duckv1.Conditions{
		{
			Type:   apis.ConditionSucceeded,
			Reason: v1beta1.PipelineRunReasonPending.String(),
		},
	}
	err := qm.AddToPendingQueue(repo, []string{PrKey(pr)})
	assert.NilError(t, err)

	sema := qm.queueMap[RepoKey(repo)]
	assert.Equal(t, len(sema.getCurrentPending()), 1)
}

func TestNewManagerForList(t *testing.T) {
	// Skip if we are running on OSX, there is a problem with ordering only happening on arm64
	skipOnOSX64(t)
	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()

	qm := NewManager(logger)

	// repository for which pipelineRun are created
	repo := newTestRepo(1)

	// first pipelineRun
	prFirst := newTestPR("first", time.Now(), nil, nil, tektonv1.PipelineRunSpec{})

	// added to queue, as there is only one should start
	started, err := qm.AddListToRunningQueue(repo, []string{PrKey(prFirst)})
	assert.NilError(t, err)
	assert.Equal(t, len(started), 1)

	// removing the running from queue
	assert.Equal(t, qm.RemoveAndTakeItemFromQueue(repo, prFirst), "")

	// adding another 2 pipelineRun, limit is 1 so this will be added to pending queue and
	// then one will be started
	prSecond := newTestPR("second", time.Now().Add(1*time.Second), nil, nil, tektonv1.PipelineRunSpec{})
	prThird := newTestPR("third", time.Now().Add(7*time.Second), nil, nil, tektonv1.PipelineRunSpec{})

	started, err = qm.AddListToRunningQueue(repo, []string{PrKey(prSecond), PrKey(prThird)})
	assert.NilError(t, err)
	assert.Equal(t, len(started), 1)
	// as per the list, 2nd must be started
	assert.Equal(t, started[0], PrKey(prSecond))

	// adding 2 more, will be going to pending queue
	prFourth := newTestPR("fourth", time.Now().Add(5*time.Second), nil, nil, tektonv1.PipelineRunSpec{})
	prFifth := newTestPR("fifth", time.Now().Add(4*time.Second), nil, nil, tektonv1.PipelineRunSpec{})

	started, err = qm.AddListToRunningQueue(repo, []string{PrKey(prFourth), PrKey(prFifth)})
	assert.NilError(t, err)
	assert.Equal(t, len(started), 0)

	// removing 2nd from queue, which means it should start 3rd
	assert.Equal(t, qm.RemoveAndTakeItemFromQueue(repo, prSecond), PrKey(prThird))

	// changing the concurrency limit to 2
	repo.Spec.ConcurrencyLimit = intPtr(2)

	prSixth := newTestPR("sixth", time.Now().Add(7*time.Second), nil, nil, tektonv1.PipelineRunSpec{})
	prSeventh := newTestPR("seventh", time.Now().Add(5*time.Second), nil, nil, tektonv1.PipelineRunSpec{})
	prEight := newTestPR("eight", time.Now().Add(4*time.Second), nil, nil, tektonv1.PipelineRunSpec{})

	started, err = qm.AddListToRunningQueue(repo, []string{PrKey(prSixth), PrKey(prSeventh), PrKey(prEight)})
	assert.NilError(t, err)
	// third is running, but limit is changed now, so one more should be moved to running
	assert.Equal(t, len(started), 1)
	assert.Equal(t, started[0], PrKey(prFourth))
}

func TestNewManagerReListing(t *testing.T) {
	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()

	qm := NewManager(logger)

	// repository for which pipelineRun are created
	repo := newTestRepo(2)

	prFirst := newTestPR("first", time.Now(), nil, nil, tektonv1.PipelineRunSpec{})
	prSecond := newTestPR("second", time.Now().Add(1*time.Second), nil, nil, tektonv1.PipelineRunSpec{})
	prThird := newTestPR("third", time.Now().Add(7*time.Second), nil, nil, tektonv1.PipelineRunSpec{})

	// added to queue, as there is only one should start
	started, err := qm.AddListToRunningQueue(repo, []string{PrKey(prFirst), PrKey(prSecond), PrKey(prThird)})
	assert.NilError(t, err)
	assert.Equal(t, len(started), 2)

	// if first is running and other pipelineRuns are reconciling
	// then adding again shouldn't have any effect
	started, err = qm.AddListToRunningQueue(repo, []string{PrKey(prFirst), PrKey(prSecond), PrKey(prThird)})
	assert.NilError(t, err)
	assert.Equal(t, len(started), 0)

	// again
	started, err = qm.AddListToRunningQueue(repo, []string{PrKey(prFirst), PrKey(prSecond), PrKey(prThird)})
	assert.NilError(t, err)
	assert.Equal(t, len(started), 0)

	// still there should only one running and 2 in pending
	assert.Equal(t, len(qm.RunningPipelineRuns(repo)), 2)
	assert.Equal(t, len(qm.QueuedPipelineRuns(repo)), 1)
	// assert.Equal(t, qm.QueuedPipelineRuns(repo)[0], "test-ns/third")

	// a new request comes
	prFourth := newTestPR("fourth", time.Now(), nil, nil, tektonv1.PipelineRunSpec{})
	prFifth := newTestPR("fifth", time.Now().Add(1*time.Second), nil, nil, tektonv1.PipelineRunSpec{})
	prSixths := newTestPR("sixth", time.Now().Add(7*time.Second), nil, nil, tektonv1.PipelineRunSpec{})

	started, err = qm.AddListToRunningQueue(repo, []string{PrKey(prFourth), PrKey(prFifth), PrKey(prSixths)})
	assert.NilError(t, err)
	assert.Equal(t, len(started), 0)

	assert.Equal(t, len(qm.RunningPipelineRuns(repo)), 2)
	assert.Equal(t, len(qm.QueuedPipelineRuns(repo)), 4)
}

func newTestRepo(limit int) *v1alpha1.Repository {
	return &v1alpha1.Repository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test-ns",
		},
		Spec: v1alpha1.RepositorySpec{
			ConcurrencyLimit: intPtr(limit),
		},
	}
}

var intPtr = func(val int) *int { return &val }

func newTestPR(name string, time time.Time, labels, annotations map[string]string, spec tektonv1.PipelineRunSpec) *tektonv1.PipelineRun {
	return &tektonv1.PipelineRun{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         "test-ns",
			CreationTimestamp: metav1.Time{Time: time},
			Labels:            labels,
			Annotations:       annotations,
		},
		Spec:   spec,
		Status: tektonv1.PipelineRunStatus{},
	}
}

func TestQueueManager_InitQueues(t *testing.T) {
	ctx, _ := rtesting.SetupFakeContext(t)
	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()
	cw := clockwork.NewFakeClock()

	startedLabel := map[string]string{
		keys.State: kubeinteraction.StateStarted,
	}
	queuedLabel := map[string]string{
		keys.State: kubeinteraction.StateQueued,
	}

	repo := newTestRepo(1)

	queuedAnnotations := map[string]string{
		keys.ExecutionOrder: "test-ns/first,test-ns/second,test-ns/third",
		keys.State:          kubeinteraction.StateQueued,
	}
	startedAnnotations := map[string]string{
		keys.ExecutionOrder: "test-ns/first,test-ns/second,test-ns/third",
		keys.State:          kubeinteraction.StateStarted,
	}
	firstPR := newTestPR("first", cw.Now(), startedLabel, startedAnnotations, tektonv1.PipelineRunSpec{})
	secondPR := newTestPR("second", cw.Now().Add(5*time.Second), queuedLabel, queuedAnnotations, tektonv1.PipelineRunSpec{
		Status: tektonv1.PipelineRunSpecStatusPending,
	})
	thirdPR := newTestPR("third", cw.Now().Add(3*time.Second), queuedLabel, queuedAnnotations, tektonv1.PipelineRunSpec{
		Status: tektonv1.PipelineRunSpecStatusPending,
	})

	tdata := testclient.Data{
		Repositories: []*v1alpha1.Repository{repo},
		PipelineRuns: []*tektonv1.PipelineRun{firstPR, secondPR, thirdPR},
	}
	stdata, _ := testclient.SeedTestData(t, ctx, tdata)

	qm := NewManager(logger)

	err := qm.InitQueues(ctx, stdata.Pipeline, stdata.PipelineAsCode)
	assert.NilError(t, err)

	// queues are built
	sema := qm.queueMap[RepoKey(repo)]
	assert.Equal(t, len(sema.getCurrentPending()), 2)
	assert.Equal(t, len(sema.getCurrentRunning()), 1)

	// now if first is completed and removed from running queue
	// then second must start as per execution order
	qm.RemoveAndTakeItemFromQueue(repo, firstPR)
	assert.Equal(t, sema.getCurrentRunning()[0], PrKey(secondPR))
	assert.Equal(t, sema.getCurrentPending()[0], PrKey(thirdPR))

	// list current running pipelineRuns for repo
	runs := qm.RunningPipelineRuns(repo)
	assert.Equal(t, len(runs), 1)
	// list current pending pipelineRuns for repo
	runs = qm.QueuedPipelineRuns(repo)
	assert.Equal(t, len(runs), 1)
}

func TestFilterPipelineRunByInProgress(t *testing.T) {
	ctx, _ := rtesting.SetupFakeContext(t)
	ns := "test-ns"

	// Create a fake Tekton client
	pipelineRuns := []*tektonv1.PipelineRun{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pr1",
				Namespace: ns,
				Annotations: map[string]string{
					keys.State: kubeinteraction.StateQueued,
				},
			},
			Spec: tektonv1.PipelineRunSpec{
				Status: tektonv1.PipelineRunSpecStatusPending,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pr2",
				Namespace: ns,
				Annotations: map[string]string{
					keys.State: kubeinteraction.StateCompleted,
				},
			},
			Spec: tektonv1.PipelineRunSpec{
				Status: tektonv1.PipelineRunSpecStatusPending,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pr3",
				Namespace: ns,
				Annotations: map[string]string{
					keys.State: kubeinteraction.StateQueued,
				},
			},
			Spec: tektonv1.PipelineRunSpec{
				Status: tektonv1.PipelineRunSpecStatusCancelled,
			},
		},
	}

	tdata := testclient.Data{
		Namespaces: []*corev1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: ns,
				},
			},
		},
		PipelineRuns: pipelineRuns,
	}

	orderList := make([]string, len(pipelineRuns))
	for i, pr := range pipelineRuns {
		orderList[i] = fmt.Sprintf("%s/%s", ns, pr.GetName())
	}
	stdata, _ := testclient.SeedTestData(t, ctx, tdata)
	filtered := FilterPipelineRunByState(ctx, stdata.Pipeline, orderList, tektonv1.PipelineRunSpecStatusPending, kubeinteraction.StateQueued)
	expected := []string{"test-ns/pr1"}
	assert.DeepEqual(t, filtered, expected)
}

func TestQueueManagerRequeueToPending(t *testing.T) {
	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()

	qm := NewManager(logger)
	repo := newTestRepo(1)
	cw := clockwork.NewFakeClock()

	// Add and acquire a PR
	pr := newTestPR("first", cw.Now(), nil, nil, tektonv1.PipelineRunSpec{})
	started, err := qm.AddListToRunningQueue(repo, []string{PrKey(pr)})
	assert.NilError(t, err)
	assert.Equal(t, len(started), 1)
	assert.Equal(t, len(qm.RunningPipelineRuns(repo)), 1)
	assert.Equal(t, len(qm.QueuedPipelineRuns(repo)), 0)

	// Re-queue the running PR back to pending
	result := qm.RequeueToPending(repo, pr)
	assert.Equal(t, result, true)
	assert.Equal(t, len(qm.RunningPipelineRuns(repo)), 0)
	assert.Equal(t, len(qm.QueuedPipelineRuns(repo)), 1)
	assert.Equal(t, qm.QueuedPipelineRuns(repo)[0], PrKey(pr))
}

func TestQueueManagerRequeueToPendingNotRunning(t *testing.T) {
	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()

	qm := NewManager(logger)
	repo := newTestRepo(1)
	cw := clockwork.NewFakeClock()

	// Try to requeue a PR that was never added
	pr := newTestPR("nonexistent", cw.Now(), nil, nil, tektonv1.PipelineRunSpec{})
	result := qm.RequeueToPending(repo, pr)
	assert.Equal(t, result, false)
}

func TestQueueManagerRequeueToPendingNoSemaphore(t *testing.T) {
	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()

	qm := NewManager(logger)
	repo := newTestRepo(1)
	cw := clockwork.NewFakeClock()

	// Try to requeue without initializing the semaphore
	pr := newTestPR("test", cw.Now(), nil, nil, tektonv1.PipelineRunSpec{})
	result := qm.RequeueToPending(repo, pr)
	assert.Equal(t, result, false)
}

func TestQueueManagerRequeueToPendingAfterFailure(t *testing.T) {
	// Skip if we are running on OSX, there is a problem with ordering only happening on arm64
	skipOnOSX64(t)
	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()

	qm := NewManager(logger)
	repo := newTestRepo(1)
	cw := clockwork.NewFakeClock()

	// Add three PRs
	prFirst := newTestPR("first", cw.Now(), nil, nil, tektonv1.PipelineRunSpec{})
	prSecond := newTestPR("second", cw.Now().Add(1*time.Second), nil, nil, tektonv1.PipelineRunSpec{})
	prThird := newTestPR("third", cw.Now().Add(2*time.Second), nil, nil, tektonv1.PipelineRunSpec{})

	started, err := qm.AddListToRunningQueue(repo, []string{PrKey(prFirst), PrKey(prSecond), PrKey(prThird)})
	assert.NilError(t, err)
	assert.Equal(t, len(started), 1)
	assert.Equal(t, started[0], PrKey(prFirst))
	assert.Equal(t, len(qm.RunningPipelineRuns(repo)), 1)
	assert.Equal(t, len(qm.QueuedPipelineRuns(repo)), 2)

	// Simulate failure: first PR fails to start, so we requeue it
	result := qm.RequeueToPending(repo, prFirst)
	assert.Equal(t, result, true)

	// Now we have all three in pending, ordered by creation time
	assert.Equal(t, len(qm.RunningPipelineRuns(repo)), 0)
	assert.Equal(t, len(qm.QueuedPipelineRuns(repo)), 3)

	// Try to start again - first should be at the front
	started, err = qm.AddListToRunningQueue(repo, []string{PrKey(prFirst), PrKey(prSecond), PrKey(prThird)})
	assert.NilError(t, err)
	assert.Equal(t, len(started), 1)
	assert.Equal(t, started[0], PrKey(prFirst))
}

func TestQueueManagerRequeueToPendingPreservesOrder(t *testing.T) {
	// Skip if we are running on OSX, there is a problem with ordering only happening on arm64
	skipOnOSX64(t)
	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()

	qm := NewManager(logger)
	repo := newTestRepo(2)
	cw := clockwork.NewFakeClock()

	// Add three PRs with different timestamps
	prFirst := newTestPR("first", cw.Now(), nil, nil, tektonv1.PipelineRunSpec{})
	prSecond := newTestPR("second", cw.Now().Add(1*time.Second), nil, nil, tektonv1.PipelineRunSpec{})
	prThird := newTestPR("third", cw.Now().Add(2*time.Second), nil, nil, tektonv1.PipelineRunSpec{})

	started, err := qm.AddListToRunningQueue(repo, []string{PrKey(prFirst), PrKey(prSecond), PrKey(prThird)})
	assert.NilError(t, err)
	assert.Equal(t, len(started), 2) // Limit is 2
	assert.Equal(t, len(qm.RunningPipelineRuns(repo)), 2)
	assert.Equal(t, len(qm.QueuedPipelineRuns(repo)), 1)

	// Re-queue second PR (which has middle timestamp)
	result := qm.RequeueToPending(repo, prSecond)
	assert.Equal(t, result, true)

	// Should have first running, second and third pending
	assert.Equal(t, len(qm.RunningPipelineRuns(repo)), 1)
	assert.Equal(t, len(qm.QueuedPipelineRuns(repo)), 2)

	// Verify both second and third are in pending (order may vary in heap)
	pending := qm.QueuedPipelineRuns(repo)
	assert.Equal(t, len(pending), 2)
	foundSecond := false
	foundThird := false
	for _, pr := range pending {
		if pr == PrKey(prSecond) {
			foundSecond = true
		}
		if pr == PrKey(prThird) {
			foundThird = true
		}
	}
	assert.Equal(t, foundSecond, true)
	assert.Equal(t, foundThird, true)

	// Verify first is still running
	running := qm.RunningPipelineRuns(repo)
	assert.Equal(t, len(running), 1)
	assert.Equal(t, running[0], PrKey(prFirst))
}

func TestQueueManagerRequeueToPendingIntegrationWithCompletion(t *testing.T) {
	// Skip if we are running on OSX, there is a problem with ordering only happening on arm64
	skipOnOSX64(t)
	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()

	qm := NewManager(logger)
	repo := newTestRepo(1)
	cw := clockwork.NewFakeClock()

	// Add two PRs
	prFirst := newTestPR("first", cw.Now(), nil, nil, tektonv1.PipelineRunSpec{})
	prSecond := newTestPR("second", cw.Now().Add(1*time.Second), nil, nil, tektonv1.PipelineRunSpec{})

	started, err := qm.AddListToRunningQueue(repo, []string{PrKey(prFirst), PrKey(prSecond)})
	assert.NilError(t, err)
	assert.Equal(t, len(started), 1)
	assert.Equal(t, started[0], PrKey(prFirst))

	// Simulate failure and requeue first PR
	result := qm.RequeueToPending(repo, prFirst)
	assert.Equal(t, result, true)

	// Both should be pending now
	assert.Equal(t, len(qm.QueuedPipelineRuns(repo)), 2)
	assert.Equal(t, len(qm.RunningPipelineRuns(repo)), 0)

	// Manually acquire first from queue (simulating retry)
	started, err = qm.AddListToRunningQueue(repo, []string{PrKey(prFirst), PrKey(prSecond)})
	assert.NilError(t, err)
	assert.Equal(t, len(started), 1)
	assert.Equal(t, started[0], PrKey(prFirst))

	// Now complete first and verify second starts
	next := qm.RemoveAndTakeItemFromQueue(repo, prFirst)
	assert.Equal(t, next, PrKey(prSecond))
	assert.Equal(t, len(qm.RunningPipelineRuns(repo)), 1)
	assert.Equal(t, len(qm.QueuedPipelineRuns(repo)), 0)
}

func TestQueueManagerRequeueToPendingByKey(t *testing.T) {
	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()

	qm := NewManager(logger)
	repo := newTestRepo(1)
	cw := clockwork.NewFakeClock()

	// Add and acquire a PR
	pr := newTestPR("first", cw.Now(), nil, nil, tektonv1.PipelineRunSpec{})
	started, err := qm.AddListToRunningQueue(repo, []string{PrKey(pr)})
	assert.NilError(t, err)
	assert.Equal(t, len(started), 1)
	assert.Equal(t, len(qm.RunningPipelineRuns(repo)), 1)
	assert.Equal(t, len(qm.QueuedPipelineRuns(repo)), 0)

	// Re-queue using just the key (simulating Get failure where we don't have PR object)
	result := qm.RequeueToPendingByKey(RepoKey(repo), PrKey(pr))
	assert.Equal(t, result, true)
	assert.Equal(t, len(qm.RunningPipelineRuns(repo)), 0)
	assert.Equal(t, len(qm.QueuedPipelineRuns(repo)), 1)
	assert.Equal(t, qm.QueuedPipelineRuns(repo)[0], PrKey(pr))
}

func TestQueueManagerRequeueToPendingByKeyInvalidRepo(t *testing.T) {
	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()

	qm := NewManager(logger)
	repo := newTestRepo(1)
	cw := clockwork.NewFakeClock()

	pr := newTestPR("test", cw.Now(), nil, nil, tektonv1.PipelineRunSpec{})

	// Try to requeue without initializing the semaphore
	result := qm.RequeueToPendingByKey(RepoKey(repo), PrKey(pr))
	assert.Equal(t, result, false)
}

func TestQueueManagerRequeueToPendingByKeyNotRunning(t *testing.T) {
	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()

	qm := NewManager(logger)
	repo := newTestRepo(1)
	cw := clockwork.NewFakeClock()

	// Initialize the semaphore by adding a PR to pending
	pr := newTestPR("first", cw.Now(), nil, nil, tektonv1.PipelineRunSpec{})
	err := qm.AddToPendingQueue(repo, []string{PrKey(pr)})
	assert.NilError(t, err)

	// Try to requeue a PR that was never acquired (not in running state)
	pr2 := newTestPR("second", cw.Now(), nil, nil, tektonv1.PipelineRunSpec{})
	result := qm.RequeueToPendingByKey(RepoKey(repo), PrKey(pr2))
	assert.Equal(t, result, false)
}

func TestRemoveAndTakeItemFromQueueCleansUpOnGetFailure(t *testing.T) {
	// Test simulates the scenario where RemoveAndTakeItemFromQueue returns a key,
	// but when we try to Get() the PipelineRun from k8s it fails (e.g., PR was deleted).
	// We need to ensure the key is removed from the running map to free the semaphore slot.
	skipOnOSX64(t)
	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()

	qm := NewManager(logger)
	repo := newTestRepo(1)
	cw := clockwork.NewFakeClock()

	// Add two PRs
	prFirst := newTestPR("first", cw.Now(), nil, nil, tektonv1.PipelineRunSpec{})
	prSecond := newTestPR("second", cw.Now().Add(1*time.Second), nil, nil, tektonv1.PipelineRunSpec{})

	started, err := qm.AddListToRunningQueue(repo, []string{PrKey(prFirst), PrKey(prSecond)})
	assert.NilError(t, err)
	assert.Equal(t, len(started), 1)
	assert.Equal(t, started[0], PrKey(prFirst))
	assert.Equal(t, len(qm.RunningPipelineRuns(repo)), 1)
	assert.Equal(t, len(qm.QueuedPipelineRuns(repo)), 1)

	// Complete first PR and get next
	next := qm.RemoveAndTakeItemFromQueue(repo, prFirst)
	assert.Equal(t, next, PrKey(prSecond))

	// At this point, second is in running map
	assert.Equal(t, len(qm.RunningPipelineRuns(repo)), 1)
	assert.Equal(t, qm.RunningPipelineRuns(repo)[0], PrKey(prSecond))

	// Simulate the scenario: Get() fails (PR was deleted from k8s)
	// The reconciler should call RemoveFromQueue to clean up
	// This simulates: _ = r.qm.RemoveFromQueue(queuepkg.RepoKey(repo), next)
	removed := qm.RemoveFromQueue(RepoKey(repo), next)
	assert.Equal(t, removed, true)

	// Verify the running map is now empty (semaphore slot freed)
	assert.Equal(t, len(qm.RunningPipelineRuns(repo)), 0)
	assert.Equal(t, len(qm.QueuedPipelineRuns(repo)), 0)

	// Add a new PR - should be able to acquire immediately since slot is free
	prThird := newTestPR("third", cw.Now().Add(2*time.Second), nil, nil, tektonv1.PipelineRunSpec{})
	started, err = qm.AddListToRunningQueue(repo, []string{PrKey(prThird)})
	assert.NilError(t, err)
	assert.Equal(t, len(started), 1)
	assert.Equal(t, started[0], PrKey(prThird))
}
