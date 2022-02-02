package sync

import (
	"fmt"
	"sync"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"go.uber.org/zap"
)

type (
	GetSyncLimit func(string) (int, error)
	IsPRDeleted  func(string) bool
)

type Manager struct {
	syncLockMap  map[string]Semaphore
	lock         *sync.Mutex
	getSyncLimit GetSyncLimit
	isPRDeleted  IsPRDeleted
	logger       *zap.SugaredLogger
}

func NewLockManager(getSyncLimit GetSyncLimit, isPRDeleted IsPRDeleted, logger *zap.SugaredLogger) *Manager {
	return &Manager{
		syncLockMap:  make(map[string]Semaphore),
		lock:         &sync.Mutex{},
		getSyncLimit: getSyncLimit,
		isPRDeleted:  isPRDeleted,
		logger:       logger,
	}
}

func (m *Manager) Register(pr *v1beta1.PipelineRun, pipelineCS versioned.Interface) error {
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

	m.logger.Info("holder: ", lock.getCurrentHolders())
	m.logger.Info("pending: ", lock.getCurrentPending())

	// start a syncer for the repo
	m.startSyncer(lockKey, pipelineCS)

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
				// TODO: do something
				// do something
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
	limit, err := m.getSyncLimit(semaphoreName)
	if err != nil {
		return nil, err
	}
	return NewSemaphore(semaphoreName, limit, m.logger), nil
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
	limit, err := m.getSyncLimit(semaphore.getName())
	if err != nil {
		return false, semaphore.getLimit(), err
	}
	return semaphore.getLimit() != limit, limit, nil
}
