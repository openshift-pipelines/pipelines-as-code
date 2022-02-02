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
	s.logger.Infof("Removed from queue: %s", holderKey)
}

func (s *PrioritySemaphore) acquire(holderKey string) bool {
	if s.semaphore.TryAcquire(1) {
		s.lockHolder[holderKey] = true
		return true
	}
	return false
}

func (p PrioritySemaphore) acquireForLatest(holderKey string) string {
	panic("implement me")
}

func (p PrioritySemaphore) tryAcquire(holderKey string) (bool, string) {
	panic("implement me")
}

func (p PrioritySemaphore) release(key string) bool {
	panic("implement me")
}

func (s *PrioritySemaphore) addToQueue(holderKey string, creationTime time.Time) {

	s.lock.Lock()
	defer s.lock.Unlock()

	if _, ok := s.lockHolder[holderKey]; ok {
		s.logger.Infof("lock already acquired by %s", holderKey)
		return
	}

	s.pending.add(holderKey, creationTime)
	s.logger.Infof("added %s to queue", holderKey)
}
