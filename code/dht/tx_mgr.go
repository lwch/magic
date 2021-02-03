package dht

import (
	"container/list"
	"encoding/binary"
	"sync"

	"github.com/lwch/magic/code/data"
)

const txBucketSize = 1024

type tx struct {
	id     string       // transaction id
	hash   hashType     // get_peers.info_hash
	remote hashType     // find_node.target
	t      data.ReqType // request type
}

type txMgr struct {
	sync.RWMutex
	list  [txBucketSize]*list.List
	count int
	max   int
}

func newTXMgr(max int) *txMgr {
	mgr := &txMgr{max: max}
	for i := 0; i < txBucketSize; i++ {
		mgr.list[i] = list.New()
	}
	return mgr
}

func (mgr *txMgr) close() {
}

func (mgr *txMgr) size() int {
	return mgr.count
}

func txHash(tx string) int {
	if len(tx) > 4 {
		bt := []byte(tx)
		var hash uint32
		for i := 0; i < len(bt)/4; i++ {
			hash += binary.BigEndian.Uint32(bt[i*4:])
		}
		return int(hash)
	}
	return 0xcccccccc
}

func (mgr *txMgr) add(id string, t data.ReqType, hash hashType, remote hashType) {
	list := mgr.list[txHash(id)%txBucketSize]
	if list.Len() >= 1000 {
		mgr.Lock()
		list.Remove(list.Front())
		mgr.count--
		mgr.Unlock()
	}
	mgr.Lock()
	list.PushBack(tx{
		id:     id,
		hash:   hash,
		remote: remote,
		t:      t,
	})
	mgr.count++
	mgr.Unlock()
}

func (mgr *txMgr) find(id string) *tx {
	l := mgr.list[txHash(id)%txBucketSize]
	var node *list.Element
	mgr.RLock()
	for node = l.Back(); node != nil; node = node.Prev() {
		if node.Value.(tx).id == id {
			break
		}
	}
	mgr.RUnlock()
	if node != nil {
		mgr.Lock()
		tx := l.Remove(node).(tx)
		mgr.count--
		mgr.Unlock()
		return &tx
	}
	return nil
}

func (mgr *txMgr) clear(limit int) {
	// mgr.Lock()
	// defer mgr.Unlock()
	// for k, tx := range mgr.txs {
	// 	if time.Now().After(tx.deadline) {
	// 		delete(mgr.txs, k)
	// 		limit--
	// 		if limit <= 0 {
	// 			return
	// 		}
	// 	}
	// }
}
