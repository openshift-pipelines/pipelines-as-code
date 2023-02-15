package sync

import (
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestNewQueueManagerForList(t *testing.T) {
	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()

	qm := NewQueueManager(logger)

	// repository for which pipelineRun are created
	repo := newTestRepo("test", 1)

	// first pipelineRun
	prFirst := newTestPR("first", time.Now(), nil, nil)

	// added to queue, as there is only one should start
	started, err := qm.AddListToQueue(repo, []string{getQueueKey(prFirst)})
	assert.NilError(t, err)
	assert.Equal(t, len(started), 1)

	// removing the running from queue
	assert.Equal(t, qm.RemoveFromQueue(repo, prFirst), "")

	// adding another 2 pipelineRun, limit is 1 so this will be added to pending queue and
	// then one will be started
	prSecond := newTestPR("second", time.Now().Add(1*time.Second), nil, nil)
	prThird := newTestPR("third", time.Now().Add(7*time.Second), nil, nil)

	started, err = qm.AddListToQueue(repo, []string{getQueueKey(prSecond), getQueueKey(prThird)})
	assert.NilError(t, err)
	assert.Equal(t, len(started), 1)
	// as per the list, 2nd must be started
	assert.Equal(t, started[0], getQueueKey(prSecond))

	// adding 2 more, will be going to pending queue
	prFourth := newTestPR("fourth", time.Now().Add(5*time.Second), nil, nil)
	prFifth := newTestPR("fifth", time.Now().Add(4*time.Second), nil, nil)

	started, err = qm.AddListToQueue(repo, []string{getQueueKey(prFourth), getQueueKey(prFifth)})
	assert.NilError(t, err)
	assert.Equal(t, len(started), 0)

	// removing 2nd from queue, which means it should start 3rd
	assert.Equal(t, qm.RemoveFromQueue(repo, prSecond), getQueueKey(prThird))

	// changing the concurrency limit to 2
	repo.Spec.ConcurrencyLimit = intPtr(2)

	prSixth := newTestPR("sixth", time.Now().Add(7*time.Second), nil, nil)
	prSeventh := newTestPR("seventh", time.Now().Add(5*time.Second), nil, nil)
	prEight := newTestPR("eight", time.Now().Add(4*time.Second), nil, nil)

	started, err = qm.AddListToQueue(repo, []string{getQueueKey(prSixth), getQueueKey(prSeventh), getQueueKey(prEight)})
	assert.NilError(t, err)
	// third is running, but limit is changed now, so one more should be moved to running
	assert.Equal(t, len(started), 1)
	assert.Equal(t, started[0], getQueueKey(prFourth))
}

func TestNewQueueManagerReListing(t *testing.T) {
	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()

	qm := NewQueueManager(logger)

	// repository for which pipelineRun are created
	repo := newTestRepo("test", 2)

	prFirst := newTestPR("first", time.Now(), nil, nil)
	prSecond := newTestPR("second", time.Now().Add(1*time.Second), nil, nil)
	prThird := newTestPR("third", time.Now().Add(7*time.Second), nil, nil)

	// added to queue, as there is only one should start
	started, err := qm.AddListToQueue(repo, []string{getQueueKey(prFirst), getQueueKey(prSecond), getQueueKey(prThird)})
	assert.NilError(t, err)
	assert.Equal(t, len(started), 2)

	// if first is running and other pipelineRuns are reconciling
	// then adding again shouldn't have any effect
	started, err = qm.AddListToQueue(repo, []string{getQueueKey(prFirst), getQueueKey(prSecond), getQueueKey(prThird)})
	assert.NilError(t, err)
	assert.Equal(t, len(started), 0)

	// again
	started, err = qm.AddListToQueue(repo, []string{getQueueKey(prFirst), getQueueKey(prSecond), getQueueKey(prThird)})
	assert.NilError(t, err)
	assert.Equal(t, len(started), 0)

	// still there should only one running and 2 in pending
	assert.Equal(t, len(qm.RunningPipelineRuns(repo)), 2)
	assert.Equal(t, len(qm.QueuedPipelineRuns(repo)), 1)
	assert.Equal(t, qm.QueuedPipelineRuns(repo)[0], "test-ns/third")

	// a new request comes
	prFourth := newTestPR("fourth", time.Now(), nil, nil)
	prFifth := newTestPR("fifth", time.Now().Add(1*time.Second), nil, nil)
	prSixths := newTestPR("sixth", time.Now().Add(7*time.Second), nil, nil)

	started, err = qm.AddListToQueue(repo, []string{getQueueKey(prFourth), getQueueKey(prFifth), getQueueKey(prSixths)})
	assert.NilError(t, err)
	assert.Equal(t, len(started), 0)

	assert.Equal(t, len(qm.RunningPipelineRuns(repo)), 2)
	assert.Equal(t, len(qm.QueuedPipelineRuns(repo)), 4)
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

func newTestPR(name string, time time.Time, labels, annotations map[string]string) *tektonv1.PipelineRun {
	return &tektonv1.PipelineRun{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         "test-ns",
			CreationTimestamp: metav1.Time{Time: time},
			Labels:            labels,
			Annotations:       annotations,
		},
		Spec:   tektonv1.PipelineRunSpec{},
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

	repo := newTestRepo("test", 1)

	annotations := map[string]string{
		keys.ExecutionOrder: "test-ns/first,test-ns/second,test-ns/third",
	}
	firstPR := newTestPR("first", cw.Now(), startedLabel, annotations)
	secondPR := newTestPR("second", cw.Now().Add(5*time.Second), queuedLabel, annotations)
	thirdPR := newTestPR("third", cw.Now().Add(3*time.Second), queuedLabel, annotations)

	tdata := testclient.Data{
		Repositories: []*v1alpha1.Repository{repo},
		PipelineRuns: []*tektonv1.PipelineRun{firstPR, secondPR, thirdPR},
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
	// then second must start as per execution order
	qm.RemoveFromQueue(repo, firstPR)
	assert.Equal(t, sema.getCurrentRunning()[0], getQueueKey(secondPR))
	assert.Equal(t, sema.getCurrentPending()[0], getQueueKey(thirdPR))

	// list current running pipelineRuns for repo
	runs := qm.RunningPipelineRuns(repo)
	assert.Equal(t, len(runs), 1)
	// list current pending pipelineRuns for repo
	runs = qm.QueuedPipelineRuns(repo)
	assert.Equal(t, len(runs), 1)
}
