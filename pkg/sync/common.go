package sync

import "time"

type Semaphore interface {
	acquireForLatest(holderKey string) string
	acquire(holderKey string) bool
	tryAcquire(holderKey string) (bool, string)
	release(key string) bool
	addToQueue(holderKey string, creationTime time.Time)
	removeFromQueue(holderKey string)
	getCurrentHolders() []string
	getCurrentPending() []string
	getName() string
	getLimit() int
	resize(n int) bool
	syncerExist() bool
	setSyncer(b bool)
}
