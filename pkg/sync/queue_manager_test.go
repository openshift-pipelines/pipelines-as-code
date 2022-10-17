package sync

import (
	"fmt"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestNewQueueManager(t *testing.T) {
	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()
	cw := clockwork.NewFakeClock()

	qm := NewQueueManager(logger)

	// repository for which pipelineRun are created
	repo := newTestRepo("test", 1)

	// first pipelineRun
	prFirst := newTestPR("first", cw.Now(), nil)

	// added to queue, as there is only one should start
	started, _, err := qm.AddToQueue(repo, prFirst)
	assert.NilError(t, err)
	assert.Equal(t, started, true)

	// adding another pipelineRun, limit is 1 so this will be added to pending queue
	prSecond := newTestPR("second", cw.Now().Add(1*time.Second), nil)

	started, msg, err := qm.AddToQueue(repo, prSecond)
	assert.NilError(t, err)
	assert.Equal(t, started, false)
	assert.Equal(t, msg, "Waiting for test-ns/test lock. Available queue status: 0/1")

	// removing first pr from running
	qm.RemoveFromQueue(repo, prFirst)

	// first is removed so the pending should be moved to running
	sema := qm.queueMap[repoKey(repo)]
	assert.Equal(t, sema.getCurrentRunning()[0], getQueueKey(prSecond))

	// updating concurrency to 2
	repo.Spec.ConcurrencyLimit = intPtr(2)

	prThird := newTestPR("third", cw.Now().Add(7*time.Second), nil)
	prFourth := newTestPR("fourth", cw.Now().Add(5*time.Second), nil)
	prFifth := newTestPR("fifth", cw.Now().Add(4*time.Second), nil)

	// Second is still running now, when third is added it should get started
	started, _, err = qm.AddToQueue(repo, prThird)
	assert.NilError(t, err)
	assert.Equal(t, started, true)

	started, _, err = qm.AddToQueue(repo, prFourth)
	assert.NilError(t, err)
	assert.Equal(t, started, false)

	started, _, err = qm.AddToQueue(repo, prFifth)
	assert.NilError(t, err)
	assert.Equal(t, started, false)

	// now if second is finished then next fifth should be started
	// as its priority i.e creation time is before fourth
	next := qm.RemoveFromQueue(repo, prSecond)
	assert.Equal(t, next, getQueueKey(prFifth))
}

func newTestRepo(name string, limit int) *v1alpha1.Repository {
	return &v1alpha1.Repository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "test-ns",
		},
		Spec: v1alpha1.RepositorySpec{
			ConcurrencyLimit: intPtr(limit),
		},
	}
}

var intPtr = func(val int) *int { return &val }

func newTestPR(name string, time time.Time, labels map[string]string) *v1beta1.PipelineRun {
	return &v1beta1.PipelineRun{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         "test-ns",
			CreationTimestamp: metav1.Time{Time: time},
			Labels:            labels,
		},
		Spec:   v1beta1.PipelineRunSpec{},
		Status: v1beta1.PipelineRunStatus{},
	}
}

func TestQueueManager_InitQueues(t *testing.T) {
	ctx, _ := rtesting.SetupFakeContext(t)
	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()
	cw := clockwork.NewFakeClock()

	startedLabel := map[string]string{
		fmt.Sprintf("%s/%s", pipelinesascode.GroupName, "state"): kubeinteraction.StateStarted,
	}
	queuedLabel := map[string]string{
		fmt.Sprintf("%s/%s", pipelinesascode.GroupName, "state"): kubeinteraction.StateQueued,
	}

	repo := newTestRepo("test", 1)

	firstPR := newTestPR("first", cw.Now(), startedLabel)
	secondPR := newTestPR("second", cw.Now().Add(5*time.Second), queuedLabel)
	thirdPR := newTestPR("third", cw.Now().Add(3*time.Second), queuedLabel)

	tdata := testclient.Data{
		Repositories: []*v1alpha1.Repository{repo},
		PipelineRuns: []*v1beta1.PipelineRun{firstPR, secondPR, thirdPR},
	}
	stdata, _ := testclient.SeedTestData(t, ctx, tdata)

	qm := NewQueueManager(logger)

	err := qm.InitQueues(ctx, stdata.Pipeline, stdata.PipelineAsCode)
	assert.NilError(t, err)

	// queues are built
	sema := qm.queueMap[repoKey(repo)]
	assert.Equal(t, len(sema.getCurrentPending()), 2)
	assert.Equal(t, len(sema.getCurrentRunning()), 1)

	// now if first is completed and removed from running queue
	// then third must start as it was created before second
	qm.RemoveFromQueue(repo, firstPR)
	assert.Equal(t, sema.getCurrentRunning()[0], getQueueKey(thirdPR))
	assert.Equal(t, sema.getCurrentPending()[0], getQueueKey(secondPR))

	// list current running pipelineRuns for repo
	runs := qm.RunningPipelineRuns(repo)
	assert.Equal(t, len(runs), 1)
	// list current pending pipelineRuns for repo
	runs = qm.QueuedPipelineRuns(repo)
	assert.Equal(t, len(runs), 1)
}
