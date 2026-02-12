package queue

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestPriorityQueue(t *testing.T) {
	// create new priority queue
	pq := &priorityQueue{itemByKey: make(map[string]*item)}

	// Priority Wise
	// item-d > item-b > item-c > item-a
	// 2 > 3 > 7 > 13
	// less priority number means early to execute

	// adding items with random priorities
	// priority is creation time hence the item with less priority number
	// will be on top of Queue
	pq.add("item-a", 13)
	pq.add("item-b", 3)
	pq.add("item-c", 7)
	pq.add("item-d", 2)

	// number of items
	assert.Equal(t, pq.Len(), 4)
	assert.Equal(t, pq.peek().key, "item-d")

	// pop from the queue
	i := pq.pop()
	assert.Equal(t, i.key, "item-d")

	// check the top most
	assert.Equal(t, pq.peek().key, "item-b")

	// items remaining
	assert.Equal(t, pq.Len(), 3)

	pq.remove("item-b")

	// check the top most
	assert.Equal(t, pq.peek().key, "item-c")

	// items remaining
	assert.Equal(t, pq.Len(), 2)

	// changing priority
	pq.add("item-a", 1)

	// check the top most
	assert.Equal(t, pq.peek().key, "item-c")
}
