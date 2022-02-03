package sync

import (
	"fmt"
	"sync"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"go.uber.org/zap"
)

type (
	GetSyncLimit func() int
)

type Manager struct {
	syncLockMap  map[string]Semaphore
	lock         *sync.Mutex
	getSyncLimit GetSyncLimit
	logger       *zap.SugaredLogger
}

func NewLockManager(logger *zap.SugaredLogger) *Manager {
	return &Manager{
		syncLockMap: make(map[string]Semaphore),
		lock:        &sync.Mutex{},
		logger:      logger,
	}
}

func (m *Manager) Register(pr *v1beta1.PipelineRun, repo *v1alpha1.Repository, pipelineCS versioned.Interface) error {

	m.getSyncLimit = func() int {
		return *repo.Spec.ConcurrencyLimit
	}

	m.lock.Lock()
	defer m.lock.Unlock()

	syncLock := GetLockName(pr)

	lockKey := syncLock.LockKey()

	// check if semaphore exist for current repo
	// if not then create
	var err error
	lock, found := m.syncLockMap[lockKey]
	if !found {
		lock, err = m.initializeSemaphore(lockKey)
		if err != nil {
			errStr := fmt.Sprintf("failed to init semaphore for repository : %s, %v", lockKey, err)
			m.logger.Error(errStr)
			return fmt.Errorf(errStr)
		}
		m.syncLockMap[lockKey] = lock
	}

	// if sync limit is changed then update size of semaphore
	m.checkAndUpdateSemaphoreSize(lock)

	lock.addToQueue(HolderKey(pr), pr.CreationTimestamp.Time)

	// start a syncer for the repo
	go m.startSyncer(lockKey, pipelineCS)

	return nil
}

func (m *Manager) startSyncer(lockKey string, pipelineCS versioned.Interface) {

	// check if syncer already exist
	m.lock.Lock()
	defer m.lock.Unlock()

	if !m.syncLockMap[lockKey].syncerExist() {

		// if syncer doesn't exist then start one
		go func() {
			err := syncer(m, lockKey, pipelineCS)

			// check if syncer exits because of any error
			if err != nil {
				m.logger.Error("syncer exited for ", lockKey)
			}

			// if no error then mark syncer as false
			m.lock.Lock()
			defer m.lock.Unlock()

			m.syncLockMap[lockKey].setSyncer(false)

			// if there are still pipelineRun in running or pending then restart
			// syncer
			if len(m.syncLockMap[lockKey].getCurrentHolders()) != 0 ||
				len(m.syncLockMap[lockKey].getCurrentPending()) != 0 {
				m.logger.Infof("restarting syncer for lock %s", lockKey)
				go m.startSyncer(lockKey, pipelineCS)
			}
		}()

		// mark syncer as true
		m.syncLockMap[lockKey].setSyncer(true)
	}
}

func (m *Manager) initializeSemaphore(semaphoreName string) (Semaphore, error) {
	return NewSemaphore(semaphoreName, m.getSyncLimit(), m.logger), nil
}

func (m *Manager) checkAndUpdateSemaphoreSize(semaphore Semaphore) {
	newLimit := m.getSyncLimit()
	if semaphore.getLimit() != newLimit {
		semaphore.resize(newLimit)
	}
}

func (m *Manager) Release(lockKey, holderKey string) {
	m.lock.Lock()
	defer m.lock.Unlock()

	if syncLockHolder, ok := m.syncLockMap[lockKey]; ok {
		syncLockHolder.release(holderKey)
		syncLockHolder.removeFromQueue(holderKey)
	}
}

func (m *Manager) Remove(lockKey, holderKey string) {
	m.lock.Lock()
	defer m.lock.Unlock()

	if syncLockHolder, ok := m.syncLockMap[lockKey]; ok {
		syncLockHolder.removeFromQueue(holderKey)
	}
}
