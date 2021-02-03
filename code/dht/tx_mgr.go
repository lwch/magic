package dht

import (
	"container/list"
	"encoding/binary"
	"sync"

	"github.com/lwch/magic/code/data"
)

type tx struct {
	id     string // transaction id
	idHash uint32
	hash   hashType     // get_peers.info_hash
	remote hashType     // find_node.target
	t      data.ReqType // request type
}

type txMgr struct {
	sync.RWMutex
	list *list.List
	max  int
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

func txHash(tx string) uint32 {
	if len(tx) > 4 {
		bt := []byte(tx)
		var hash uint32
		for i := 0; i < len(bt)/4; i++ {
			hash += binary.BigEndian.Uint32(bt[i*4:])
		}
		return hash
	}
	return 0xcccccccc
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
		idHash: txHash(id),
		hash:   hash,
		remote: remote,
		t:      t,
	})
	mgr.Unlock()
}

func (mgr *txMgr) find(id string) *tx {
	idHash := txHash(id)
	var node *list.Element
	mgr.RLock()
	for node = mgr.list.Back(); node != nil; node = node.Prev() {
		if node.Value.(tx).idHash == idHash &&
			node.Value.(tx).id == id {
			break
		}
	}
	mgr.RUnlock()
	if node != nil {
		mgr.Lock()
		tx := mgr.list.Remove(node).(tx)
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
