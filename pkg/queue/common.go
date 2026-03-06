package queue

import (
	"time"
)

type Semaphore interface {
	acquire(string) bool
	acquireLatest() string
	tryAcquire(string) (bool, string)
	release(string) bool
	resize(int) bool
	addToQueue(string, time.Time) bool
	addToPendingQueue(string, time.Time) bool
	removeFromQueue(string)
	requeueToPending(string, time.Time) bool
	getName() string
	getLimit() int
	getCurrentRunning() []string
	getCurrentPending() []string
}
