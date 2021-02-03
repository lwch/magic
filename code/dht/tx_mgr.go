package dht

import (
	"sync"
	"time"

	"github.com/lwch/magic/code/data"
)

type tx struct {
	id       string       // transaction id
	hash     hashType     // get_peers.info_hash
	remote   hashType     // find_node.target
	t        data.ReqType // request type
	deadline time.Time
}

type txMgr struct {
	sync.RWMutex
	txs     map[string]tx
	timeout time.Duration
}

func newTXMgr(timeout time.Duration) *txMgr {
	return &txMgr{txs: make(map[string]tx), timeout: timeout}
}

func (mgr *txMgr) close() {
}

func (mgr *txMgr) size() int {
	return len(mgr.txs)
}

func (mgr *txMgr) add(id string, t data.ReqType, hash hashType, remote hashType) {
	mgr.Lock()
	defer mgr.Unlock()
	mgr.txs[id] = tx{
		id:       id,
		hash:     hash,
		remote:   remote,
		t:        t,
		deadline: time.Now().Add(mgr.timeout),
	}
}

func (mgr *txMgr) find(id string) *tx {
	mgr.RLock()
	tx, ok := mgr.txs[id]
	mgr.RUnlock()
	if ok {
		mgr.Lock()
		delete(mgr.txs, id)
		mgr.Unlock()
		return &tx
	}
	return nil
}

func (mgr *txMgr) clear(limit int) {
	mgr.Lock()
	defer mgr.Unlock()
	for k, tx := range mgr.txs {
		if time.Now().After(tx.deadline) {
			delete(mgr.txs, k)
			limit--
			if limit <= 0 {
				return
			}
		}
	}
}
