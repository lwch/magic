package dht

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"sync"
)

type bucket struct {
	sync.RWMutex
	prefix hashType
	nodes  []*node
	leaf   [2]*bucket
	bits   int
}

func (bk *bucket) isLeaf() bool {
	bk.RLock()
	defer bk.RUnlock()
	return bk.leaf[0] == nil && bk.leaf[1] == nil
}

func (bk *bucket) addNode(n *node, k int) bool {
	bk.Lock()
	defer bk.Unlock()
	if bk.exists(n.id) {
		return false
	}
	if len(bk.nodes) >= k {
		loopSplit(bk, k)
		target := bk.searchAdd(n)
		if target.exists(n.id) {
			return false
		}
		target.nodes = append(target.nodes, n)
		return true
	}
	bk.nodes = append(bk.nodes, n)
	return true
}

func loopSplit(bk *bucket, k int) {
	if bk.bits > len(bk.prefix)*8-k {
		panic(fmt.Errorf("too large: %d", bk.bits))
	}
	bk.split()
	if len(bk.leaf[0].nodes) >= k {
		loopSplit(bk.leaf[0], k)
	}
	if len(bk.leaf[1].nodes) >= k {
		loopSplit(bk.leaf[1], k)
	}
}

func (bk *bucket) exists(id hashType) bool {
	for _, node := range bk.nodes {
		if bytes.Equal(node.id[:], id[:]) {
			return true
		}
	}
	return false
}

func (bk *bucket) searchAdd(n *node) *bucket {
	if bk.leaf[0] == nil && bk.leaf[1] == nil {
		return bk
	}
	return bk.leaf[n.id.bit(bk.bits+1)].searchAdd(n)
}

func (bk *bucket) split() {
	var id hashType
	copy(id[:], bk.prefix[:])
	if bk.leaf[0] == nil {
		bk.leaf[0] = newBucket(id, bk.bits+1)
	}
	if bk.leaf[1] == nil {
		bt := bk.bits / 8
		bit := bk.bits % 8
		id[bt] |= 1 << (7 - bit)
		bk.leaf[1] = newBucket(id, bk.bits+1)
	}
	for _, node := range bk.nodes {
		if bk.leaf[0].equalBits(node.id) {
			bk.leaf[0].nodes = append(bk.leaf[0].nodes, node)
		} else {
			bk.leaf[1].nodes = append(bk.leaf[1].nodes, node)
		}
	}
	bk.nodes = nil
}

func (bk *bucket) equalBits(id hashType) bool {
	bt := bk.bits / 8
	bit := bk.bits % 8
	for i := 0; i < bt; i++ {
		if bk.prefix[i]^id[i] > 0 {
			return false
		}
	}
	a := bk.prefix[bt] >> (8 - bit)
	b := id[bt] >> (8 - bit)
	if a^b > 0 {
		return false
	}
	return true
}

func newBucket(prefix hashType, bits int) *bucket {
	return &bucket{
		prefix: prefix,
		bits:   bits,
	}
}

type table struct {
	sync.RWMutex
	dht  *DHT
	root *bucket
	k    int
	size int

	// runtime
	ctx    context.Context
	cancel context.CancelFunc
}

func newTable(dht *DHT, k int) *table {
	tb := &table{
		dht:  dht,
		root: newBucket(emptyHash, 0),
		k:    k,
	}
	tb.ctx, tb.cancel = context.WithCancel(context.Background())
	return tb
}

func (t *table) close() {
	t.cancel()
}

func (t *table) discovery() {
}

func (t *table) add(n *node) bool {
	t.Lock()
	defer t.Unlock()
	next := t.root
	for idx := 0; idx < len(n.id)*8; idx++ {
		if next.isLeaf() {
			return next.addNode(n, t.k)
		}
		next = next.leaf[n.id.bit(idx)]
	}
	return false
}

func (t *table) remove(n *node) {
	n.close()
}

func (t *table) findAddr(addr net.Addr) *node {
	return nil
}

func (t *table) findID(id hashType) *node {
	return nil
}

func (t *table) neighbor(id hashType, n int) []*node {
	return nil
}
