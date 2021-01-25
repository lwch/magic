package dht

type tx struct {
	id     string
	hash   hashType
	remote idType
	t      string
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

func (mgr *txMgr) add(id, t string, hash hashType, remote idType) {
	mgr.txs[mgr.idx%len(mgr.txs)] = tx{
		id:     id,
		hash:   hash,
		remote: remote,
		t:      t,
	}
	mgr.idx++
}
