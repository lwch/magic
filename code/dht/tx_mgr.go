package dht

import (
	"crypto/md5"
	"encoding/binary"
	"sync"
	"time"

	"github.com/lwch/hashmap"
	"github.com/lwch/magic/code/data"
)

type tx struct {
	id       string       // transaction id
	hash     hashType     // get_peers.info_hash
	remote   hashType     // find_node.target
	t        data.ReqType // request type
	deadline time.Time
}

type txSlice struct {
	sync.Mutex
	data []tx
	size uint64
}

func (s *txSlice) Make(size uint64) {
	s.data = make([]tx, size)
}

func (s *txSlice) Resize(size uint64) {
	s.Lock()
	defer s.Unlock()
	data := make([]tx, size)
	copy(data, s.data)
	s.data = data
}

func (s *txSlice) Size() uint64 {
	return uint64(s.size)
}

func (s *txSlice) Cap() uint64 {
	return uint64(len(s.data))
}

func (s *txSlice) Hash(key interface{}) uint64 {
	sum := md5.Sum([]byte(key.(string)))
	a := binary.BigEndian.Uint64(sum[:])
	b := binary.BigEndian.Uint64(sum[8:])
	return a + b
}

func (s *txSlice) KeyEqual(idx uint64, key interface{}) bool {
	node := s.data[int(idx)%len(s.data)]
	return node.id == key.(string)
}

func (s *txSlice) Empty(idx uint64) bool {
	node := s.data[int(idx)%len(s.data)]
	return len(node.id) == 0 && len(node.t) == 0
}

func (s *txSlice) Set(idx uint64, key, value interface{}, deadtime time.Time, update bool) bool {
	target := &s.data[int(idx)%len(s.data)]
	if len(target.id) > 0 || len(target.t) > 0 {
		return false
	}
	v := value.(tx)
	target.id = v.id
	target.hash = v.hash
	target.remote = v.remote
	target.t = v.t
	target.deadline = deadtime
	if !update {
		s.size++
	}
	return true
}

func (s *txSlice) Get(idx uint64) interface{} {
	node := s.data[int(idx)%len(s.data)]
	return &node
}

func (s *txSlice) Reset(idx uint64) {
	node := &s.data[int(idx)%len(s.data)]
	node.id = ""
	node.t = ""
	s.size--
}

func (s *txSlice) Timeout(idx uint64) bool {
	node := s.data[int(idx)%len(s.data)]
	return time.Now().After(node.deadline)
}

type txMgr struct {
	txs *hashmap.Map
}

func newTXMgr(timeout time.Duration) *txMgr {
	return &txMgr{txs: hashmap.New(&txSlice{}, 1000, 5, timeout)}
}

func (mgr *txMgr) close() {
}

func (mgr *txMgr) size() int {
	return int(mgr.txs.Size())
}

func (mgr *txMgr) add(id string, t data.ReqType, hash hashType, remote hashType) {
	mgr.txs.Set(id, tx{
		id:     id,
		hash:   hash,
		remote: remote,
		t:      t,
	})
}

func (mgr *txMgr) find(id string) *tx {
	node := mgr.txs.Get(id)
	if node == nil {
		return nil
	}
	mgr.txs.Remove(id)
	return node.(*tx)
}

func (mgr *txMgr) clear() {
	mgr.txs.Clear()
}
