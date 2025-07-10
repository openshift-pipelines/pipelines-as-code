package sync

import (
	"fmt"
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

func TestConcurrentMethodCallsDeadlock(t *testing.T) {
	// Test concurrent calls to different methods to ensure no deadlocks
	repo := newSemaphore("concurrent-methods-test", 2)
	cw := clockwork.NewFakeClock()

	// Add some initial items
	assert.Equal(t, repo.addToQueue("key1", cw.Now()), true)
	assert.Equal(t, repo.addToQueue("key2", cw.Now().Add(1*time.Second)), true)
	assert.Equal(t, repo.addToQueue("key3", cw.Now().Add(2*time.Second)), true)

	// Channels for synchronization
	done := make(chan bool, 10)
	startSignal := make(chan bool)

	// Start multiple goroutines calling different methods concurrently
	methods := []func(){
		func() { repo.tryAcquire("key1"); done <- true },
		func() { repo.tryAcquire("key2"); done <- true },
		func() { repo.acquireLatest(); done <- true },
		func() { repo.release("key1"); done <- true },
		func() { repo.addToQueue("key4", cw.Now().Add(3*time.Second)); done <- true },
		func() { repo.addToPendingQueue("key5", cw.Now().Add(4*time.Second)); done <- true },
		func() { repo.removeFromQueue("key3"); done <- true },
		func() { repo.getCurrentRunning(); done <- true },
		func() { repo.getCurrentPending(); done <- true },
		func() { repo.resize(3); done <- true },
	}

	// Start all goroutines
	for _, method := range methods {
		go func(m func()) {
			<-startSignal
			m()
		}(method)
	}

	// Signal all goroutines to start
	close(startSignal)

	// Wait for all to complete with timeout
	completed := 0
	timeout := time.After(5 * time.Second)

	for completed < len(methods) {
		select {
		case <-done:
			completed++
		case <-timeout:
			t.Fatalf("DEADLOCK DETECTED: Only %d/%d methods completed within 5 seconds", completed, len(methods))
		}
	}

	t.Logf("SUCCESS: All %d concurrent method calls completed without deadlock", len(methods))
}

func TestResizeDeadlockScenarios(t *testing.T) {
	// Test resize operations concurrent with other operations
	repo := newSemaphore("resize-deadlock-test", 2)
	cw := clockwork.NewFakeClock()

	// Add items and acquire some
	assert.Equal(t, repo.addToQueue("key1", cw.Now()), true)
	assert.Equal(t, repo.addToQueue("key2", cw.Now().Add(1*time.Second)), true)
	repo.tryAcquire("key1")
	repo.tryAcquire("key2")

	done := make(chan bool, 4)
	startSignal := make(chan bool)

	// Concurrent resize operations
	go func() {
		<-startSignal
		repo.resize(1) // Downsize
		done <- true
	}()

	go func() {
		<-startSignal
		repo.resize(4) // Upsize
		done <- true
	}()

	// Concurrent with other operations
	go func() {
		<-startSignal
		repo.addToQueue("key3", cw.Now().Add(2*time.Second))
		done <- true
	}()

	go func() {
		<-startSignal
		repo.release("key1")
		done <- true
	}()

	// Start all operations
	close(startSignal)

	// Wait for completion
	completed := 0
	timeout := time.After(5 * time.Second)

	for completed < 4 {
		select {
		case <-done:
			completed++
		case <-timeout:
			t.Fatal("DEADLOCK DETECTED: Resize operations did not complete within 5 seconds")
		}
	}

	t.Log("SUCCESS: Concurrent resize operations completed without deadlock")
}

func TestHighConcurrencyStressTest(t *testing.T) {
	// Stress test with many concurrent operations
	repo := newSemaphore("stress-test", 5)
	cw := clockwork.NewFakeClock()

	const numGoroutines = 50
	const numOperations = 10

	done := make(chan bool, numGoroutines)
	startSignal := make(chan bool)

	// Start many goroutines performing random operations
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			<-startSignal
			defer func() { done <- true }()

			for j := 0; j < numOperations; j++ {
				key := fmt.Sprintf("key-%d-%d", id, j)

				// Perform random operations
				switch j % 6 {
				case 0:
					repo.addToQueue(key, cw.Now().Add(time.Duration(j)*time.Millisecond))
				case 1:
					repo.tryAcquire(key)
				case 2:
					repo.acquireLatest()
				case 3:
					repo.release(key)
				case 4:
					repo.removeFromQueue(key)
				case 5:
					repo.getCurrentRunning()
				}
			}
		}(i)
	}

	// Start all goroutines
	close(startSignal)

	// Wait for all to complete
	completed := 0
	timeout := time.After(10 * time.Second)

	for completed < numGoroutines {
		select {
		case <-done:
			completed++
		case <-timeout:
			t.Fatalf("DEADLOCK DETECTED: Only %d/%d goroutines completed within 10 seconds", completed, numGoroutines)
		}
	}

	t.Logf("SUCCESS: %d goroutines with %d operations each completed without deadlock", numGoroutines, numOperations)
}

func TestPriorityQueueDeadlockScenarios(t *testing.T) {
	// Test potential deadlocks in priority queue operations
	repo := newSemaphore("priority-queue-deadlock-test", 3)
	cw := clockwork.NewFakeClock()

	// Pre-populate the queue to avoid race conditions
	assert.Equal(t, repo.addToQueue("key1", cw.Now()), true)
	assert.Equal(t, repo.addToQueue("key2", cw.Now().Add(1*time.Second)), true)
	assert.Equal(t, repo.addToQueue("key3", cw.Now().Add(2*time.Second)), true)

	done := make(chan bool, 6)
	startSignal := make(chan bool)

	// Concurrent priority queue operations - but safer ones
	operations := []func(){
		func() { repo.addToQueue("key4", cw.Now().Add(3*time.Second)); done <- true },
		func() { repo.addToQueue("key5", cw.Now().Add(4*time.Second)); done <- true },
		func() { repo.getCurrentPending(); done <- true },
		func() { repo.getCurrentRunning(); done <- true },
		func() { repo.tryAcquire("key1"); done <- true },
		func() { repo.release("key1"); done <- true },
	}

	// Start all operations concurrently
	for _, op := range operations {
		go func(operation func()) {
			<-startSignal
			operation()
		}(op)
	}

	// Signal start
	close(startSignal)

	// Wait for completion
	completed := 0
	timeout := time.After(5 * time.Second)

	for completed < len(operations) {
		select {
		case <-done:
			completed++
		case <-timeout:
			t.Fatal("DEADLOCK DETECTED: Priority queue operations did not complete within 5 seconds")
		}
	}

	t.Log("SUCCESS: Priority queue operations completed without deadlock")
}

func TestSemaphoreExhaustionScenario(t *testing.T) {
	// Test scenario where semaphore is exhausted and operations block
	repo := newSemaphore("exhaustion-test", 1)
	cw := clockwork.NewFakeClock()

	// Fill the semaphore
	assert.Equal(t, repo.addToQueue("key1", cw.Now()), true)
	acquired, _ := repo.tryAcquire("key1")
	assert.Equal(t, acquired, true)

	// Add more items to pending queue
	assert.Equal(t, repo.addToQueue("key2", cw.Now().Add(1*time.Second)), true)
	assert.Equal(t, repo.addToQueue("key3", cw.Now().Add(2*time.Second)), true)

	done := make(chan bool, 3)
	startSignal := make(chan bool)

	// Try to acquire when semaphore is exhausted
	go func() {
		<-startSignal
		repo.tryAcquire("key2") // Should not block, should return false
		done <- true
	}()

	go func() {
		<-startSignal
		repo.acquireLatest() // Should not block, should return empty
		done <- true
	}()

	// Release and then try operations
	go func() {
		<-startSignal
		time.Sleep(100 * time.Millisecond) // Let other operations try first
		repo.release("key1")
		done <- true
	}()

	// Start all operations
	close(startSignal)

	// Wait for completion
	completed := 0
	timeout := time.After(3 * time.Second)

	for completed < 3 {
		select {
		case <-done:
			completed++
		case <-timeout:
			t.Fatal("BLOCKING DETECTED: Semaphore exhaustion caused operations to block")
		}
	}

	t.Log("SUCCESS: Semaphore exhaustion handled without blocking")
}

func TestEdgeCaseDeadlocks(t *testing.T) {
	// Test various edge cases that might cause deadlocks
	repo := newSemaphore("edge-case-test", 2)
	cw := clockwork.NewFakeClock()

	done := make(chan bool, 8)
	startSignal := make(chan bool)

	// Edge case operations
	edgeCases := []func(){
		// Empty queue operations
		func() { repo.acquireLatest(); done <- true },
		func() { repo.removeFromQueue("nonexistent"); done <- true },
		func() { repo.release("nonexistent"); done <- true },
		func() { repo.tryAcquire("nonexistent"); done <- true },

		// Duplicate operations
		func() {
			repo.addToQueue("duplicate", cw.Now())
			repo.addToQueue("duplicate", cw.Now()) // Should not add twice
			done <- true
		},

		// Rapid resize operations
		func() {
			repo.resize(1)
			repo.resize(5)
			repo.resize(2)
			done <- true
		},

		// Operations on running items
		func() {
			repo.addToQueue("running", cw.Now())
			repo.tryAcquire("running")
			repo.addToQueue("running", cw.Now()) // Should fail
			done <- true
		},

		// Mixed operations
		func() {
			repo.getCurrentRunning()
			repo.getCurrentPending()
			repo.getLimit()
			repo.getName()
			done <- true
		},
	}

	// Start all edge case operations
	for _, edgeCase := range edgeCases {
		go func(operation func()) {
			<-startSignal
			operation()
		}(edgeCase)
	}

	// Signal start
	close(startSignal)

	// Wait for completion
	completed := 0
	timeout := time.After(5 * time.Second)

	for completed < len(edgeCases) {
		select {
		case <-done:
			completed++
		case <-timeout:
			t.Fatal("DEADLOCK DETECTED: Edge case operations did not complete within 5 seconds")
		}
	}

	t.Log("SUCCESS: All edge case operations completed without deadlock")
}

func TestTryAcquireRaceConditionBug(t *testing.T) {
	// This test specifically triggers the race condition bug in tryAcquire
	// The bug occurs when tryAcquire is called on a key that's not in the queue
	repo := newSemaphore("race-condition-test", 1)

	// Don't add any items to the queue initially

	done := make(chan bool, 10)
	startSignal := make(chan bool)

	// Start multiple goroutines trying to acquire non-existent keys
	for i := 0; i < 10; i++ {
		go func(id int) {
			<-startSignal
			// Try to acquire a key that doesn't exist in the queue
			// This should trigger the race condition
			key := fmt.Sprintf("nonexistent-key-%d", id)
			_, _ = repo.tryAcquire(key)
			done <- true
		}(i)
	}

	// Signal start
	close(startSignal)

	// Wait for completion
	completed := 0
	timeout := time.After(5 * time.Second)

	for completed < 10 {
		select {
		case <-done:
			completed++
		case <-timeout:
			t.Fatal("BUG DETECTED: tryAcquire race condition caused panic or deadlock")
		}
	}

	t.Log("Test completed - but this might have triggered the race condition bug")
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
