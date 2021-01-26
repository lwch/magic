package dht

import (
	"context"
	"sync"
	"time"

	"github.com/lwch/magic/code/data"
)

type tx struct {
	id      string       // transaction id
	hash    hashType     // get_peers.info_hash
	remote  hashType     // find_node.target
	t       data.ReqType // request type
	created time.Time
}

type txMgr struct {
	sync.RWMutex
	txs     map[string]tx
	timeout time.Duration

	// runtime
	ctx    context.Context
	cancel context.CancelFunc
}

func newTXMgr(timeout time.Duration) *txMgr {
	mgr := &txMgr{txs: make(map[string]tx), timeout: timeout}
	mgr.ctx, mgr.cancel = context.WithCancel(context.Background())
	go mgr.clear()
	return mgr
}

func (mgr *txMgr) close() {
	mgr.cancel()
}

func (mgr *txMgr) size() int {
	return len(mgr.txs)
}

func (mgr *txMgr) clear() {
	clear := func() {
		nodes := make([]*tx, 0, len(mgr.txs))
		mgr.RLock()
		for _, tx := range mgr.txs {
			if time.Since(tx.created) >= mgr.timeout {
				nodes = append(nodes, &tx)
			}
		}
		mgr.RUnlock()
		mgr.Lock()
		for _, node := range nodes {
			delete(mgr.txs, node.id)
		}
		mgr.Unlock()
	}
	t := time.NewTicker(10 * time.Second)
	for {
		select {
		case <-t.C:
			clear()
		case <-mgr.ctx.Done():
			return
		}
	}
}

func (mgr *txMgr) add(id string, t data.ReqType, hash hashType, remote hashType) {
	mgr.Lock()
	defer mgr.Unlock()
	mgr.txs[id] = tx{
		id:      id,
		hash:    hash,
		remote:  remote,
		t:       t,
		created: time.Now(),
	}
}

func (mgr *txMgr) find(tx string) *tx {
	mgr.Lock()
	defer mgr.Unlock()
	if ret, ok := mgr.txs[tx]; ok {
		delete(mgr.txs, tx)
		return &ret
	}
	return nil
}
