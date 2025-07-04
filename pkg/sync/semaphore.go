package sync

import (
	"fmt"
	"sync"
	"time"

	sema "golang.org/x/sync/semaphore"
)

// prioritySemaphore implements a semaphore with priority queue for managing concurrent access
// to resources. It maintains a queue of pending requests and tracks currently running operations.
type prioritySemaphore struct {
	name      string
	limit     int
	pending   *priorityQueue
	running   map[string]bool
	semaphore *sema.Weighted
	lock      *sync.RWMutex // Changed to RWMutex for better read performance
}

var _ Semaphore = &prioritySemaphore{}

// newSemaphore creates a new priority semaphore with the given name and concurrency limit.
func newSemaphore(name string, limit int) *prioritySemaphore {
	if limit <= 0 {
		limit = 1 // Ensure minimum limit
	}
	return &prioritySemaphore{
		name:      name,
		limit:     limit,
		pending:   &priorityQueue{itemByKey: make(map[string]*item)},
		semaphore: sema.NewWeighted(int64(limit)),
		running:   make(map[string]bool),
		lock:      &sync.RWMutex{},
	}
}

// getName returns the semaphore's name.
func (s *prioritySemaphore) getName() string {
	return s.name
}

// getLimit returns the current concurrency limit.
func (s *prioritySemaphore) getLimit() int {
	return s.limit
}

// getCurrentPending returns a slice of keys currently in the pending queue.
func (s *prioritySemaphore) getCurrentPending() []string {
	s.lock.RLock()
	defer s.lock.RUnlock()
	keys := make([]string, 0, len(s.pending.items))
	for _, item := range s.pending.items {
		keys = append(keys, item.key)
	}
	return keys
}

// getCurrentRunning returns a slice of keys currently running.
func (s *prioritySemaphore) getCurrentRunning() []string {
	s.lock.RLock()
	defer s.lock.RUnlock()
	keys := make([]string, 0, len(s.running))
	for k := range s.running {
		keys = append(keys, k)
	}
	return keys
}

// resize attempts to resize the semaphore to the new limit.
// Returns true if the resize was successful, false otherwise.
func (s *prioritySemaphore) resize(n int) bool {
	if n <= 0 {
		return false
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	currentRunning := len(s.running)
	acquireCount := currentRunning
	if currentRunning > n {
		acquireCount = n
	}

	newSemaphore := sema.NewWeighted(int64(n))
	// Only proceed if we can acquire the required slots
	if !newSemaphore.TryAcquire(int64(acquireCount)) {
		return false
	}
	s.semaphore = newSemaphore
	s.limit = n
	return true
}

// removeFromQueue removes a key from the pending queue.
func (s *prioritySemaphore) removeFromQueue(key string) {
	if key == "" {
		return
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	s.pending.remove(key)
}

// addToQueue adds a key to the pending queue with the given creation time.
// Returns true if the key was added, false if it was already present.
func (s *prioritySemaphore) addToQueue(key string, creationTime time.Time) bool {
	if key == "" {
		return false
	}
	// Validate creation time is not zero
	if creationTime.IsZero() {
		return false
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.running[key] || s.pending.isPending(key) {
		return false
	}
	s.pending.add(key, creationTime.UnixNano())
	return true
}

// addToPendingQueue is an alias for addToQueue for interface compatibility.
func (s *prioritySemaphore) addToPendingQueue(key string, creationTime time.Time) bool {
	return s.addToQueue(key, creationTime)
}

// acquireLatest attempts to acquire the next available slot from the pending queue.
// Returns the key of the acquired item, or empty string if none available.
func (s *prioritySemaphore) acquireLatest() string {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.pending.Len() == 0 {
		return ""
	}
	if len(s.running) >= s.limit {
		return ""
	}
	nextItem := s.pending.peek()
	if nextItem == nil {
		return ""
	}
	// TryAcquire should not block, but we're being defensive
	if s.semaphore.TryAcquire(1) {
		_ = s.pending.pop()
		s.running[nextItem.key] = true
		return nextItem.key
	}
	return ""
}

// release releases a key from the running set and releases the semaphore slot.
// Returns true if the key was released, false if it wasn't running.
func (s *prioritySemaphore) release(key string) bool {
	if key == "" {
		return false
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	if !s.running[key] {
		return false
	}

	delete(s.running, key)
	if len(s.running) < s.limit {
		s.semaphore.Release(1)
	}
	return true
}

// tryAcquire attempts to acquire a semaphore slot for the specified key.
// Returns (true, "") if acquired, (false, message) if waiting.
func (s *prioritySemaphore) tryAcquire(key string) (bool, string) {
	if key == "" {
		return false, "Invalid key"
	}

	s.lock.Lock()
	defer s.lock.Unlock()
	if s.running[key] {
		return true, ""
	}
	if len(s.running) >= s.limit {
		waitingMsg := fmt.Sprintf("Waiting for %s lock. Limit reached: %d/%d", s.name, len(s.running), s.limit)
		return false, waitingMsg
	}
	if s.pending.Len() > 0 {
		nextItem := s.pending.peek()
		if nextItem == nil || key != nextItem.key {
			waitingMsg := fmt.Sprintf("Waiting for %s lock. Available queue status: %d/%d", s.name, s.limit-len(s.running), s.limit)
			return false, waitingMsg
		}
		// Remove from pending queue before acquiring
		s.pending.pop()
	}
	if s.semaphore.TryAcquire(1) {
		s.running[key] = true
		return true, ""
	}
	// If we couldn't acquire, we need to add the item back to pending queue
	// This is a rare edge case but important for correctness
	if s.pending.Len() == 0 || s.pending.peek() == nil || s.pending.peek().key != key {
		// Only add back if it's not already there
		s.pending.add(key, 0) // Use 0 priority since we don't have the original timestamp
	}
	waitingMsg := fmt.Sprintf("Waiting for %s lock. Available queue status: %d/%d", s.name, s.limit-len(s.running), s.limit)
	return false, waitingMsg
}

// acquire is a helper method that attempts to acquire a semaphore slot for a key.
// This method assumes the lock is already held.
func (s *prioritySemaphore) acquire(key string) bool {
	if key == "" {
		return false
	}
	if s.semaphore.TryAcquire(1) {
		s.running[key] = true
		return true
	}
	return false
}
