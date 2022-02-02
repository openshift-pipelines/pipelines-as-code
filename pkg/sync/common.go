package sync

import "time"

type Semaphore interface {
	acquireForLatest() string
	acquire(holderKey string) bool

	addToQueue(holderKey string, creationTime time.Time)
	removeFromQueue(holderKey string)

	getCurrentHolders() []string
	getCurrentPending() []string
	getName() string
	getLimit() int

	release(key string) bool
	resize(n int) bool

	syncerExist() bool
	setSyncer(b bool)
}
