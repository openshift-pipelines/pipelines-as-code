package sync

import (
	"fmt"
	"strconv"
	"sync"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"go.uber.org/zap"
)

type (
	GetSyncLimit func(string) int
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

	m.getSyncLimit = func(s string) int {
		// check if sync limit is set in repository
		// if not then return 1 by default
		limit, ok := repo.Annotations["pipelinesascode.tekton.dev/concurrencyLimit"]
		if ok {
			i, err := strconv.Atoi(limit)
			if err != nil {
				return 1
			}
			return i
		}
		return 1
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
	err = m.checkAndUpdateSemaphoreSize(lock)
	if err != nil {
		errStr := fmt.Sprintf("failed to update semaphore size for lock %v, %v ", lock.getName(), err)
		m.logger.Error(errStr)
		return fmt.Errorf(errStr)
	}

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
				// TODO: should we restart it again?
			}

			// if no error then mark syncer as false
			m.lock.Lock()
			defer m.lock.Unlock()

			m.syncLockMap[lockKey].setSyncer(false)
		}()

		// mark syncer as true
		m.syncLockMap[lockKey].setSyncer(true)
	}
}

func (m *Manager) initializeSemaphore(semaphoreName string) (Semaphore, error) {
	return NewSemaphore(semaphoreName, m.getSyncLimit(semaphoreName), m.logger), nil
}

func (m *Manager) checkAndUpdateSemaphoreSize(semaphore Semaphore) error {
	changed, newLimit, err := m.isSemaphoreSizeChanged(semaphore)
	if err != nil {
		return err
	}
	if changed {
		semaphore.resize(newLimit)
	}
	return nil
}

func (m *Manager) isSemaphoreSizeChanged(semaphore Semaphore) (bool, int, error) {
	limit := m.getSyncLimit(semaphore.getName())
	return semaphore.getLimit() != limit, limit, nil
}

func (m *Manager) Release(lockKey, holderKey string) {
	m.lock.Lock()
	defer m.lock.Unlock()

	if syncLockHolder, ok := m.syncLockMap[lockKey]; ok {
		syncLockHolder.release(holderKey)
		syncLockHolder.removeFromQueue(holderKey)
	}
}
