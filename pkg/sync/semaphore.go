package sync

import (
	"sync"
	"time"

	"go.uber.org/zap"
	sema "golang.org/x/sync/semaphore"
)

type PrioritySemaphore struct {
	name       string
	limit      int
	pending    *priorityQueue
	semaphore  *sema.Weighted
	lockHolder map[string]bool
	lock       *sync.Mutex
	syncer     bool
	logger     *zap.SugaredLogger
}

var _ Semaphore = &PrioritySemaphore{}

func NewSemaphore(name string, limit int, logger *zap.SugaredLogger) *PrioritySemaphore {
	return &PrioritySemaphore{
		name:       name,
		limit:      limit,
		pending:    &priorityQueue{itemByKey: make(map[string]*item)},
		semaphore:  sema.NewWeighted(int64(limit)),
		lockHolder: make(map[string]bool),
		lock:       &sync.Mutex{},
		logger:     logger,
	}
}

func (s *PrioritySemaphore) getName() string {
	return s.name
}

func (s *PrioritySemaphore) getLimit() int {
	return s.limit
}

func (s *PrioritySemaphore) syncerExist() bool {
	return s.syncer
}

func (s *PrioritySemaphore) setSyncer(status bool) {
	s.syncer = status
}

func (s *PrioritySemaphore) getCurrentPending() []string {
	var keys []string
	for _, item := range s.pending.items {
		keys = append(keys, item.key)
	}
	return keys
}

func (s *PrioritySemaphore) getCurrentHolders() []string {
	var keys []string
	for k := range s.lockHolder {
		keys = append(keys, k)
	}
	return keys
}

func (s *PrioritySemaphore) resize(n int) bool {
	s.lock.Lock()
	defer s.lock.Unlock()

	cur := len(s.lockHolder)
	// downward case, acquired n locks
	if cur > n {
		cur = n
	}

	semaphore := sema.NewWeighted(int64(n))
	status := semaphore.TryAcquire(int64(cur))
	if status {
		s.logger.Infof("%s semaphore resized from %d to %d", s.name, cur, n)
		s.semaphore = semaphore
		s.limit = n
	}
	return status
}

func (s *PrioritySemaphore) removeFromQueue(holderKey string) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.pending.remove(holderKey)
	s.logger.Infof("removed from queue: %s for semaphore : %s ", holderKey, s.name)
}

func (s *PrioritySemaphore) acquire(holderKey string) bool {
	if s.semaphore.TryAcquire(1) {
		s.lockHolder[holderKey] = true
		return true
	}
	return false
}

func (s *PrioritySemaphore) acquireForLatest() string {

	s.lock.Lock()
	defer s.lock.Unlock()

	if s.pending.Len() == 0 {
		return ""
	}

	ready := s.pending.peek()

	if s.semaphore.TryAcquire(1) {
		_ = s.pending.pop()
		s.lockHolder[ready.key] = true
		s.logger.Infof("acquired lock for %s in %s", ready.key, s.name)
	}

	return ready.key
}

func (s *PrioritySemaphore) release(key string) bool {
	s.lock.Lock()
	defer s.lock.Unlock()

	if _, ok := s.lockHolder[key]; ok {

		delete(s.lockHolder, key)

		// When semaphore resized downward
		// Remove the excess holders from map once the done.
		if len(s.lockHolder) >= s.limit {
			return true
		}

		s.semaphore.Release(1)
		availableLocks := s.limit - len(s.lockHolder)
		s.logger.Infof("lock has been released by %s in semaphore %s. Available locks: %d", key, s.name, availableLocks)
	}
	return true
}

func (s *PrioritySemaphore) addToQueue(holderKey string, creationTime time.Time) {

	s.lock.Lock()
	defer s.lock.Unlock()

	if _, ok := s.lockHolder[holderKey]; ok {
		s.logger.Infof("%s lock already acquired by %s", s.name, holderKey)
		return
	}

	s.pending.add(holderKey, creationTime)
	s.logger.Infof("added %s in queue for %s", holderKey, s.name)
}
