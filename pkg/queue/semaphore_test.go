package queue

import (
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"gotest.tools/v3/assert"
)

func TestNewSemaphore(t *testing.T) {
	repo := newSemaphore("test", 1)
	cw := clockwork.NewFakeClock()

	assert.Equal(t, repo.getName(), "test")
	assert.Equal(t, repo.getLimit(), 1)

	// add elements
	// randomly adding elements, the element with the less priority
	// must execute first
	assert.Equal(t, repo.addToQueue("C", cw.Now().Add(5*time.Second)), true)
	assert.Equal(t, repo.addToQueue("A", cw.Now()), true)
	assert.Equal(t, repo.addToQueue("B", cw.Now().Add(1*time.Second)), true)

	// start the topmost, which would be A
	acquired, msg := repo.tryAcquire("A")
	assert.Equal(t, acquired, true)
	assert.Equal(t, msg, "")

	// hit again
	acquired, _ = repo.tryAcquire("A")
	assert.Equal(t, acquired, true)

	// try acquiring B, but limit is 1 so should fail
	acquired, _ = repo.tryAcquire("B")
	assert.Equal(t, acquired, false)

	// C is back in the queue should also fail
	acquired, _ = repo.tryAcquire("C")
	assert.Equal(t, acquired, false)

	assert.Equal(t, len(repo.getCurrentRunning()), 1)
	assert.Equal(t, len(repo.getCurrentPending()), 2)

	// adding element to Queue which is running
	// nothing should happen
	assert.Equal(t, repo.addToQueue("A", cw.Now().Add(5*time.Second)), false)

	// A is done
	repo.release("A")
	repo.removeFromQueue("A")

	assert.Equal(t, len(repo.getCurrentRunning()), 0)
	assert.Equal(t, len(repo.getCurrentPending()), 2)

	// start the next
	assert.Equal(t, repo.acquireLatest(), "B")

	assert.Equal(t, len(repo.getCurrentRunning()), 1)
	assert.Equal(t, len(repo.getCurrentPending()), 1)

	// B is done
	repo.release("B")
	repo.removeFromQueue("B")

	// resize to 2
	repo.resize(2)

	// now add new elements
	assert.Equal(t, repo.addToQueue("D", cw.Now().Add(8*time.Second)), true)
	assert.Equal(t, repo.addToQueue("E", cw.Now().Add(6*time.Second)), true)
	assert.Equal(t, repo.addToQueue("F", cw.Now().Add(7*time.Second)), true)

	// queue already have C in it
	// now the queue must have C > E > F > D
	// C being on the top

	// start the next
	assert.Equal(t, repo.acquireLatest(), "C")
	assert.Equal(t, repo.acquireLatest(), "E")

	// size is 2 now if we try to acquire again it will return empty
	assert.Equal(t, repo.acquireLatest(), "")

	// resize back to 1
	repo.resize(1)
	assert.Equal(t, repo.getLimit(), 1)

	assert.Equal(t, len(repo.getCurrentRunning()), 2)
	assert.Equal(t, len(repo.getCurrentPending()), 2)

	// try to start next but it shouldn't as 2 are still running
	assert.Equal(t, repo.acquireLatest(), "")

	repo.release("C")
	repo.removeFromQueue("C")

	// try again to start next but it shouldn't as 1 is
	// still running
	assert.Equal(t, repo.acquireLatest(), "")

	repo.release("E")
	repo.removeFromQueue("E")

	// empty the pending Queue
	repo.removeFromQueue("F")
	repo.removeFromQueue("D")

	assert.Equal(t, repo.acquireLatest(), "")
}

func TestTryAcquireDeadlockScenario(t *testing.T) {
	// This test ensures concurrent access to tryAcquire works without deadlocks
	repo := newSemaphore("deadlock-test", 1)
	cw := clockwork.NewFakeClock()

	// Add an item to the queue
	assert.Equal(t, repo.addToQueue("key1", cw.Now()), true)

	// Create channels for synchronization
	firstStarted := make(chan bool)
	secondStarted := make(chan bool)
	firstDone := make(chan bool)
	secondDone := make(chan bool)

	// First goroutine: try to acquire the key
	go func() {
		firstStarted <- true
		<-secondStarted // Wait for second goroutine to also start
		acquired, _ := repo.tryAcquire("key1")
		firstDone <- acquired
	}()

	// Second goroutine: try to acquire the same key concurrently
	go func() {
		<-firstStarted // Wait for first goroutine to start
		secondStarted <- true
		acquired, _ := repo.tryAcquire("key1")
		secondDone <- acquired
	}()

	// Wait for both results with a timeout
	select {
	case result1 := <-firstDone:
		select {
		case result2 := <-secondDone:
			// If we get here, no deadlock occurred
			// Both should succeed since the same key can be acquired multiple times
			// if it's already running (see line 138 in tryAcquire)
			assert.Equal(t, result1, true)
			assert.Equal(t, result2, true)
		case <-time.After(5 * time.Second):
			t.Fatal("Deadlock detected: second goroutine did not complete within 5 seconds")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Deadlock detected: first goroutine did not complete within 5 seconds")
	}
}

func TestTryAcquireDeadlockTimeout(t *testing.T) {
	// This test should hang (timeout) if the deadlock bug is present
	// It simulates the scenario where tryAcquire calls acquire() while holding the lock
	repo := newSemaphore("deadlock-test", 1)
	cw := clockwork.NewFakeClock()

	// Add an item to the queue
	assert.Equal(t, repo.addToQueue("key1", cw.Now()), true)

	done := make(chan struct{})
	go func() {
		defer close(done)
		// This would hang if tryAcquire calls acquire() while holding the lock
		repo.tryAcquire("key1")
	}()

	select {
	case <-done:
		// Success: no deadlock
	case <-time.After(2 * time.Second):
		t.Fatal("Deadlock detected: tryAcquire did not return within 2 seconds")
	}
}

func TestDeadlockDetectionRecursiveMutex(t *testing.T) {
	// This test would detect a deadlock if tryAcquire were to call acquire()
	// which would cause a recursive mutex lock (tryAcquire holds lock, then acquire tries to get same lock)
	repo := newSemaphore("recursive-deadlock-test", 1)
	cw := clockwork.NewFakeClock()

	// Add an item to the queue
	assert.Equal(t, repo.addToQueue("key1", cw.Now()), true)

	// Channel to signal completion
	done := make(chan bool, 1)

	// Start a goroutine that would deadlock if tryAcquire calls acquire
	go func() {
		defer func() { done <- true }()

		// This should complete without hanging
		// If tryAcquire calls acquire, it would deadlock here because:
		// 1. tryAcquire acquires s.lock
		// 2. tryAcquire calls acquire
		// 3. acquire tries to acquire s.lock again (same goroutine, same mutex)
		// 4. Deadlock - goroutine waits for itself
		_, _ = repo.tryAcquire("key1")
	}()

	// Wait for completion with timeout
	select {
	case <-done:
		// Success - no deadlock
		t.Log("No deadlock detected - tryAcquire completed successfully")
	case <-time.After(3 * time.Second):
		t.Fatal("DEADLOCK DETECTED: tryAcquire did not complete within 3 seconds - likely recursive mutex lock")
	}
}

func TestDeadlockDetectionConcurrentTryAcquire(t *testing.T) {
	// This test detects deadlocks in concurrent tryAcquire calls
	repo := newSemaphore("concurrent-deadlock-test", 1)
	cw := clockwork.NewFakeClock()

	// Add items to the queue
	assert.Equal(t, repo.addToQueue("key1", cw.Now()), true)
	assert.Equal(t, repo.addToQueue("key2", cw.Now().Add(1*time.Second)), true)

	// Channels for synchronization
	goroutine1Done := make(chan bool, 1)
	goroutine2Done := make(chan bool, 1)
	startSignal := make(chan bool, 1)

	// First goroutine
	go func() {
		defer func() { goroutine1Done <- true }()
		<-startSignal // Wait for start signal
		_, _ = repo.tryAcquire("key1")
	}()

	// Second goroutine
	go func() {
		defer func() { goroutine2Done <- true }()
		<-startSignal // Wait for start signal
		_, _ = repo.tryAcquire("key2")
	}()

	// Start both goroutines simultaneously
	close(startSignal)

	// Wait for both to complete with timeout
	timeout := time.After(3 * time.Second)
	completed := 0

	for completed < 2 {
		select {
		case <-goroutine1Done:
			completed++
		case <-goroutine2Done:
			completed++
		case <-timeout:
			t.Fatal("DEADLOCK DETECTED: Concurrent tryAcquire calls did not complete within 3 seconds")
		}
	}

	t.Log("No deadlock detected - concurrent tryAcquire calls completed successfully")
}

func TestTryAcquireConcurrentAccess(t *testing.T) {
	// Test concurrent access to tryAcquire to ensure no deadlocks occur
	repo := newSemaphore("concurrent-test", 2)
	cw := clockwork.NewFakeClock()

	// Add multiple items to the queue
	assert.Equal(t, repo.addToQueue("key1", cw.Now()), true)
	assert.Equal(t, repo.addToQueue("key2", cw.Now().Add(1*time.Second)), true)
	assert.Equal(t, repo.addToQueue("key3", cw.Now().Add(2*time.Second)), true)

	// Try to acquire each key in order, simulating concurrent but ordered access
	acquired1, _ := repo.tryAcquire("key1")
	acquired2, _ := repo.tryAcquire("key2")
	acquired3, _ := repo.tryAcquire("key3")

	assert.Equal(t, acquired1, true)
	assert.Equal(t, acquired2, true)
	assert.Equal(t, acquired3, false)
	assert.Equal(t, len(repo.getCurrentRunning()), 2)
}

func TestRequeueToPending(t *testing.T) {
	repo := newSemaphore("test-requeue", 1)
	cw := clockwork.NewFakeClock()

	// Add and acquire a key
	assert.Equal(t, repo.addToQueue("key1", cw.Now()), true)
	assert.Equal(t, repo.acquireLatest(), "key1")
	assert.Equal(t, len(repo.getCurrentRunning()), 1)
	assert.Equal(t, len(repo.getCurrentPending()), 0)

	// Re-queue the running key back to pending
	result := repo.requeueToPending("key1", cw.Now())
	assert.Equal(t, result, true)
	assert.Equal(t, len(repo.getCurrentRunning()), 0)
	assert.Equal(t, len(repo.getCurrentPending()), 1)
	assert.Equal(t, repo.getCurrentPending()[0], "key1")

	// Verify it can be acquired again
	assert.Equal(t, repo.acquireLatest(), "key1")
	assert.Equal(t, len(repo.getCurrentRunning()), 1)
	assert.Equal(t, len(repo.getCurrentPending()), 0)
}

func TestRequeueToPendingNotRunning(t *testing.T) {
	repo := newSemaphore("test-requeue", 1)
	cw := clockwork.NewFakeClock()

	// Try to requeue a key that's not running
	result := repo.requeueToPending("nonexistent", cw.Now())
	assert.Equal(t, result, false)
	assert.Equal(t, len(repo.getCurrentRunning()), 0)
	assert.Equal(t, len(repo.getCurrentPending()), 0)
}

func TestRequeueToPendingPreservesOrder(t *testing.T) {
	repo := newSemaphore("test-requeue-order", 2)
	cw := clockwork.NewFakeClock()

	// Add multiple keys with different timestamps
	baseTime := cw.Now()
	assert.Equal(t, repo.addToQueue("key1", baseTime), true)
	assert.Equal(t, repo.addToQueue("key2", baseTime.Add(1*time.Second)), true)
	assert.Equal(t, repo.addToQueue("key3", baseTime.Add(2*time.Second)), true)

	// Acquire first two
	assert.Equal(t, repo.acquireLatest(), "key1")
	assert.Equal(t, repo.acquireLatest(), "key2")
	assert.Equal(t, len(repo.getCurrentRunning()), 2)
	assert.Equal(t, len(repo.getCurrentPending()), 1)

	// Re-queue key2 with its original timestamp
	result := repo.requeueToPending("key2", baseTime.Add(1*time.Second))
	assert.Equal(t, result, true)
	assert.Equal(t, len(repo.getCurrentRunning()), 1)
	assert.Equal(t, len(repo.getCurrentPending()), 2)

	// Pending queue should maintain order: key1 is still running, so pending should be [key2, key3]
	pending := repo.getCurrentPending()
	assert.Equal(t, len(pending), 2)
	// key2 should come before key3 since it has earlier timestamp
	assert.Equal(t, pending[0], "key2")
	assert.Equal(t, pending[1], "key3")
}

func TestRequeueToPendingReleasesSemaphore(t *testing.T) {
	repo := newSemaphore("test-requeue-semaphore", 1)
	cw := clockwork.NewFakeClock()

	// Add and acquire first key
	assert.Equal(t, repo.addToQueue("key1", cw.Now()), true)
	assert.Equal(t, repo.acquireLatest(), "key1")

	// Add second key - should go to pending since limit is 1
	assert.Equal(t, repo.addToQueue("key2", cw.Now().Add(1*time.Second)), true)
	assert.Equal(t, len(repo.getCurrentPending()), 1)

	// Try to acquire key2 - should fail since semaphore is full
	assert.Equal(t, repo.acquireLatest(), "")

	// Re-queue key1 back to pending
	result := repo.requeueToPending("key1", cw.Now())
	assert.Equal(t, result, true)

	// Now key2 should be acquirable since semaphore slot was released
	assert.Equal(t, repo.acquireLatest(), "key1") // key1 has earlier timestamp
	assert.Equal(t, len(repo.getCurrentRunning()), 1)
	assert.Equal(t, len(repo.getCurrentPending()), 1)
}

func TestRequeueToPendingWithResize(t *testing.T) {
	repo := newSemaphore("test-requeue-resize", 2)
	cw := clockwork.NewFakeClock()

	// Add and acquire two keys
	assert.Equal(t, repo.addToQueue("key1", cw.Now()), true)
	assert.Equal(t, repo.addToQueue("key2", cw.Now().Add(1*time.Second)), true)
	assert.Equal(t, repo.acquireLatest(), "key1")
	assert.Equal(t, repo.acquireLatest(), "key2")
	assert.Equal(t, len(repo.getCurrentRunning()), 2)

	// Resize down to 1
	repo.resize(1)
	assert.Equal(t, repo.getLimit(), 1)

	// Re-queue key2 - since running count (2) >= limit (1), semaphore should not be released
	result := repo.requeueToPending("key2", cw.Now().Add(1*time.Second))
	assert.Equal(t, result, true)
	assert.Equal(t, len(repo.getCurrentRunning()), 1)
	assert.Equal(t, len(repo.getCurrentPending()), 1)

	// key1 is still running, and since we're over limit, no new acquisition should work
	assert.Equal(t, repo.acquireLatest(), "")
}

func TestRequeueToPendingAlreadyPending(t *testing.T) {
	repo := newSemaphore("test-requeue-duplicate", 1)
	cw := clockwork.NewFakeClock()

	// Add and acquire a key
	assert.Equal(t, repo.addToQueue("key1", cw.Now()), true)
	assert.Equal(t, repo.acquireLatest(), "key1")

	// Manually add key1 to pending (this shouldn't happen in practice)
	repo.pending.add("key1", cw.Now().UnixNano())

	// Try to requeue - should fail since it's already pending
	result := repo.requeueToPending("key1", cw.Now())
	assert.Equal(t, result, false)

	// key1 should be removed from running but not re-added to pending
	assert.Equal(t, len(repo.getCurrentRunning()), 0)
	assert.Equal(t, len(repo.getCurrentPending()), 1)
}

func TestRequeueToPendingConcurrent(t *testing.T) {
	repo := newSemaphore("test-requeue-concurrent", 2)
	cw := clockwork.NewFakeClock()

	// Add and acquire two keys
	assert.Equal(t, repo.addToQueue("key1", cw.Now()), true)
	assert.Equal(t, repo.addToQueue("key2", cw.Now().Add(1*time.Second)), true)
	assert.Equal(t, repo.acquireLatest(), "key1")
	assert.Equal(t, repo.acquireLatest(), "key2")

	// Channels for synchronization
	done1 := make(chan bool, 1)
	done2 := make(chan bool, 1)

	// Concurrently requeue both keys
	go func() {
		result := repo.requeueToPending("key1", cw.Now())
		done1 <- result
	}()

	go func() {
		result := repo.requeueToPending("key2", cw.Now().Add(1*time.Second))
		done2 <- result
	}()

	// Wait for both with timeout
	timeout := time.After(3 * time.Second)
	completed := 0
	for completed < 2 {
		select {
		case result := <-done1:
			assert.Equal(t, result, true)
			completed++
		case result := <-done2:
			assert.Equal(t, result, true)
			completed++
		case <-timeout:
			t.Fatal("Concurrent requeue operations did not complete within 3 seconds")
		}
	}

	// Both should be back in pending
	assert.Equal(t, len(repo.getCurrentRunning()), 0)
	assert.Equal(t, len(repo.getCurrentPending()), 2)
}
