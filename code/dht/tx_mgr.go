package dht

import (
	"container/list"
	"sync"

	"github.com/lwch/magic/code/data"
)

type tx struct {
	id     string       // transaction id
	hash   hashType     // get_peers.info_hash
	remote hashType     // find_node.target
	t      data.ReqType // request type
}

type txMgr struct {
	sync.RWMutex
	list   *list.List
	filter uint64
	max    int
}

func newTXMgr(max int) *txMgr {
	return &txMgr{
		list: list.New(),
		max:  max,
	}
}

func (mgr *txMgr) close() {
}

func (mgr *txMgr) size() int {
	return mgr.list.Len()
}

func (mgr *txMgr) add(id string, t data.ReqType, hash hashType, remote hashType) {
	if mgr.size() >= mgr.max {
		mgr.Lock()
		mgr.list.Remove(mgr.list.Front())
		mgr.Unlock()
	}
	mgr.Lock()
	mgr.list.PushBack(tx{
		id:     id,
		hash:   hash,
		remote: remote,
		t:      t,
	})
	mgr.Unlock()
}

func (mgr *txMgr) find(id string) *tx {
	mgr.RLock()
	defer mgr.RUnlock()
	for node := mgr.list.Back(); node != nil; node = node.Prev() {
		if node.Value.(tx).id == id {
			tx := mgr.list.Remove(node).(tx)
			return &tx
		}
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
