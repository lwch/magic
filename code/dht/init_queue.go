package dht

import (
	"sync"
	"time"
)

type queueData struct {
	tx       string
	n        *node
	deadline time.Time
}

type initQueue struct {
	sync.RWMutex
	data map[string]queueData
}

func newInitQueue() *initQueue {
	return &initQueue{
		data: make(map[string]queueData),
	}
}

func (q *initQueue) push(tx string, n *node) {
	q.Lock()
	defer q.Unlock()
	q.data[tx] = queueData{
		tx:       tx,
		n:        n,
		deadline: time.Now().Add(10 * time.Second),
	}
}

func (q *initQueue) find(tx string) *node {
	q.RLock()
	data, ok := q.data[tx]
	q.RUnlock()
	if time.Now().After(data.deadline) {
		ok = false
	}
	q.Lock()
	delete(q.data, tx)
	q.Unlock()
	if ok {
		return data.n
	}
	return nil
}

func (q *initQueue) unset(tx string) {
	q.Lock()
	defer q.Unlock()
	delete(q.data, tx)
}
