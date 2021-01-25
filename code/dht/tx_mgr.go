package dht

import "github.com/lwch/magic/code/data"

type tx struct {
	id     string       // transaction id
	hash   hashType     // get_peers.info_hash
	remote hashType     // find_node.target
	t      data.ReqType // request type
}

type txMgr struct {
	idx int
	txs []tx
}

func newTXMgr(max int) *txMgr {
	return &txMgr{txs: make([]tx, max)}
}

func (mgr *txMgr) close() {
}

func (mgr *txMgr) add(id string, t data.ReqType, hash hashType, remote hashType) {
	mgr.txs[mgr.idx%len(mgr.txs)] = tx{
		id:     id,
		hash:   hash,
		remote: remote,
		t:      t,
	}
	mgr.idx++
}

func (mgr *txMgr) find(tx string) *tx {
	size := mgr.idx
	if mgr.idx >= len(mgr.txs) {
		size = len(mgr.txs)
	}
	for i := 0; i < size; i++ {
		if mgr.txs[i].id == tx {
			return &mgr.txs[i]
		}
	}
	return nil
}
