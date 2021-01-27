package dht

import (
	"crypto/md5"
	"encoding/binary"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lwch/hashmap"
)

type queueData struct {
	tx       string
	n        *node
	deadline time.Time
}

type queueSlice struct {
	sync.Mutex
	data []queueData
	size int64
}

func (s *queueSlice) Make(size uint64) {
	s.data = make([]queueData, size)
}

func (s *queueSlice) Resize(size uint64) {
	s.Lock()
	defer s.Unlock()
	data := make([]queueData, size)
	copy(data, s.data)
	s.data = data
}

func (s *queueSlice) Size() uint64 {
	return uint64(s.size)
}

func (s *queueSlice) Cap() uint64 {
	return uint64(len(s.data))
}

func (s *queueSlice) Hash(key interface{}) uint64 {
	sum := md5.Sum([]byte(key.(string)))
	a := binary.BigEndian.Uint64(sum[:])
	b := binary.BigEndian.Uint64(sum[8:])
	return a + b
}

func (s *queueSlice) KeyEqual(idx uint64, key interface{}) bool {
	node := s.data[int(idx)%len(s.data)]
	return node.tx == key.(string)
}

func (s *queueSlice) Empty(idx uint64) bool {
	node := s.data[int(idx)%len(s.data)]
	return len(node.tx) == 0
}

func (s *queueSlice) Set(idx uint64, key, value interface{}, deadtime time.Time, update bool) bool {
	target := &s.data[int(idx)%len(s.data)]
	target.tx = key.(string)
	target.n = value.(*node)
	target.deadline = deadtime
	if !update {
		atomic.AddInt64(&s.size, 1)
	}
	return true
}

func (s *queueSlice) Get(idx uint64) interface{} {
	node := s.data[int(idx)%len(s.data)]
	if s.Timeout(idx) {
		return nil
	}
	return node.n
}

func (s *queueSlice) Reset(idx uint64) {
	node := &s.data[int(idx)%len(s.data)]
	node.tx = ""
	node.n = nil
	node.deadline = time.Unix(0, 0)
	atomic.AddInt64(&s.size, -1)
}

func (s *queueSlice) Timeout(idx uint64) bool {
	node := s.data[int(idx)%len(s.data)]
	return time.Now().After(node.deadline)
}

type initQueue struct {
	sync.RWMutex
	data *hashmap.Map // tx => *node
}

func newInitQueue(size int) *initQueue {
	return &initQueue{
		data: hashmap.New(&queueSlice{}, uint64(size), 10, 1000, 10*time.Second),
	}
}

func (q *initQueue) push(tx string, n *node) {
	q.Lock()
	defer q.Unlock()
	q.data.Set(tx, n)
}

func (q *initQueue) find(tx string) *node {
	q.RLock()
	defer q.RUnlock()
	n := q.data.Get(tx)
	if n == nil {
		return nil
	}
	return n.(*node)
}

func (q *initQueue) unset(tx string) {
	q.Lock()
	defer q.Unlock()
	q.data.Remove(tx)
}
