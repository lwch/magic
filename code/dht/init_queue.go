package dht

import "sync"

type initQueue struct {
	sync.RWMutex
	data map[string]*node
}

func newInitQueue() *initQueue {
	return &initQueue{data: make(map[string]*node)}
}

func (q *initQueue) push(tx string, n *node) {
	q.Lock()
	defer q.Unlock()
	q.data[tx] = n
}

func (q *initQueue) find(tx string) *node {
	q.RLock()
	defer q.RUnlock()
	return q.data[tx]
}

func (q *initQueue) unset(tx string) {
	q.Lock()
	defer q.Unlock()
	delete(q.data, tx)
}
