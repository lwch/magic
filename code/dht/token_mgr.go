package dht

import "github.com/lwch/magic/code/data"

type token struct {
	tk   string   // token value
	from hashType // node id
	hash hashType // get_peers.info_hash
}

type tokenMgr struct {
	idx    int
	tokens []token
}

func newTokenMgr(max int) *tokenMgr {
	return &tokenMgr{tokens: make([]token, max)}
}

func (mgr *tokenMgr) close() {
}

func (mgr *tokenMgr) new(hash, from hashType) string {
	tk := data.Rand(16)
	mgr.tokens[mgr.idx%len(mgr.tokens)] = token{
		tk:   tk,
		from: from,
		hash: hash,
	}
	mgr.idx++
	return tk
}

func (mgr *tokenMgr) add(tk string, hash, from hashType) {
	mgr.tokens[mgr.idx%len(mgr.tokens)] = token{
		tk:   tk,
		from: from,
		hash: hash,
	}
	mgr.idx++
}

func (mgr *tokenMgr) find(tk string) *token {
	size := mgr.idx
	if mgr.idx >= len(mgr.tokens) {
		size = len(mgr.tokens)
	}
	for i := 0; i < size; i++ {
		if mgr.tokens[i].tk == tk {
			return &mgr.tokens[i]
		}
	}
	return nil
}
