package dht

import (
	"bytes"
	"container/list"
	"context"
	"net"
	"sync"
	"time"

	"github.com/lwch/magic/code/data"
	"github.com/lwch/magic/code/logging"
)

const nodeTimeout = time.Minute
const nodeSendPing = 10 * time.Second

type bucket struct {
	sync.RWMutex
	prefix hashType
	nodes  *list.List
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
		// TODO: update
		return false
	}
	if bk.nodes.Len() >= k {
		loopSplit(bk, k)
		target := bk.search(n.id)
		if target.exists(n.id) {
			// TODO: update
			return false
		}
		target.nodes.PushBack(n)
		return true
	}
	bk.nodes.PushBack(n)
	return true
}

func loopSplit(bk *bucket, k int) {
	bk.split()
	if bk.leaf[0].nodes.Len() >= k {
		loopSplit(bk.leaf[0], k)
	}
	if bk.leaf[1].nodes.Len() >= k {
		loopSplit(bk.leaf[1], k)
	}
}

func (bk *bucket) exists(id hashType) bool {
	for n := bk.nodes.Front(); n != nil; n = n.Next() {
		if bytes.Equal(n.Value.(*node).id[:], id[:]) {
			return true
		}
	}
	return false
}

func (bk *bucket) search(id hashType) *bucket {
	if bk.leaf[0] == nil && bk.leaf[1] == nil {
		return bk
	}
	return bk.leaf[id.bit(bk.bits)].search(id)
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
		if bt == 20 {
			// TODO: panic debug
			var ids []string
			var equals []bool
			for n := bk.nodes.Front(); n != nil; n = n.Next() {
				ids = append(ids, n.Value.(*node).id.String())
				equals = append(equals, bk.equalBits(n.Value.(*node).id))
			}
			logging.Info("overflow: prefix=%s, ids=%v, equals=%v", bk.prefix.String(), ids, equals)
		}
		id[bt] |= 1 << (7 - bit)
		bk.leaf[1] = newBucket(id, bk.bits+1)
	}
	for n := bk.nodes.Front(); n != nil; n = n.Next() {
		node := n.Value.(*node)
		if bk.leaf[0].equalBits(node.id) {
			bk.leaf[0].nodes.PushBack(node)
		} else {
			bk.leaf[1].nodes.PushBack(node)
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

func (bk *bucket) clearTimeout() []*node {
	bk.Lock()
	defer bk.Unlock()
	removed := make([]*node, 0, bk.nodes.Len())
	for n := bk.nodes.Front(); n != nil; n = n.Next() {
		element := n.Value.(*node)
		since := time.Since(element.updated)
		if !element.isBootstrap && since >= nodeTimeout {
			logging.Debug("timeout: %s", element.id.String())
			removed = append(removed, bk.nodes.Remove(n).(*node))
			continue
		} else if since >= nodeSendPing {
			tx := element.sendPing()
			element.dht.tx.add(tx, data.TypePing, emptyHash, emptyHash)
		}
	}
	return removed
}

func (bk *bucket) getNodes() []*node {
	bk.RLock()
	defer bk.RUnlock()
	ret := make([]*node, 0, bk.nodes.Len())
	for n := bk.nodes.Front(); n != nil; n = n.Next() {
		ret = append(ret, n.Value.(*node))
	}
	return ret
}

func newBucket(prefix hashType, bits int) *bucket {
	return &bucket{
		prefix: prefix,
		nodes:  list.New(),
		bits:   bits,
	}
}

type table struct {
	sync.RWMutex
	dht       *DHT
	root      *bucket
	addrIndex map[string]*node
	k         int
	size      int

	// runtime
	ctx    context.Context
	cancel context.CancelFunc
}

func newTable(dht *DHT, k int) *table {
	tb := &table{
		dht:       dht,
		root:      newBucket(emptyHash, 0),
		addrIndex: make(map[string]*node),
		k:         k,
	}
	tb.ctx, tb.cancel = context.WithCancel(context.Background())
	go func() {
		tk := time.Tick(time.Second)
		for {
			select {
			case <-tk:
				dht.Nodes <- tb.size
			case <-tb.ctx.Done():
				return
			}
		}
	}()
	return tb
}

func (t *table) close() {
	t.cancel()
}

func (t *table) discoverySend(bk *bucket, limit *int) {
	if *limit <= 0 {
		return
	}
	if bk == nil {
		return
	}
	if bk.isLeaf() {
		for n := bk.nodes.Front(); n != nil; n = n.Next() {
			n.Value.(*node).sendDiscovery()
		}
		*limit -= bk.nodes.Len()
		t.Lock()
		for _, node := range bk.clearTimeout() {
			delete(t.addrIndex, node.addr.String())
			node.close()
			t.size--
		}
		t.Unlock()
		return
	}
	t.dht.even++
	if t.dht.even%2 == 0 {
		t.discoverySend(bk.leaf[0], limit)
		t.discoverySend(bk.leaf[1], limit)
	} else {
		t.discoverySend(bk.leaf[1], limit)
		t.discoverySend(bk.leaf[0], limit)
	}
}

func (t *table) discovery(limit int) {
	t.discoverySend(t.root, &limit)
}

func (t *table) add(n *node) bool {
	t.Lock()
	defer t.Unlock()
	next := t.root
	for idx := 0; idx < len(n.id)*8; idx++ {
		if next.isLeaf() {
			ok := next.addNode(n, t.k)
			if ok {
				t.addrIndex[n.addr.String()] = n
				t.size++
			}
			return ok
		}
		next = next.leaf[n.id.bit(idx)]
	}
	return false
}

func (t *table) remove(n *node) {
	n.close()

	t.Lock()
	defer t.Unlock()
	bk := t.root.search(n.id)
	for nd := bk.nodes.Front(); nd != nil; nd = nd.Next() {
		node := nd.Value.(*node)
		if !node.id.equal(n.id) {
			continue
		}
		delete(t.addrIndex, n.addr.String())
		bk.nodes.Remove(nd)
		node.close()
		t.size--
	}
}

func (t *table) findAddr(addr net.Addr) *node {
	t.RLock()
	data := t.addrIndex[addr.String()]
	t.RUnlock()
	if data == nil {
		return nil
	}
	// free node
	if t.dht.even%2 == 0 {
		bk := t.root.search(data.id)
		t.Lock()
		for _, node := range bk.clearTimeout() {
			delete(t.addrIndex, node.addr.String())
			node.close()
			t.size--
		}
		t.Unlock()
	}
	t.dht.even++
	return data
}

func (t *table) findID(id hashType) *node {
	t.RLock()
	bk := t.root.search(id)
	t.RUnlock()
	// free node
	defer func() {
		t.dht.even++
		if t.dht.even%2 == 0 {
			return
		}
		t.Lock()
		for _, node := range bk.clearTimeout() {
			delete(t.addrIndex, node.addr.String())
			node.close()
			t.size--
		}
		t.Unlock()
	}()
	for n := bk.nodes.Front(); n != nil; n = n.Next() {
		node := n.Value.(*node)
		if node.id.equal(id) {
			return node
		}
	}
	return nil
}

func (t *table) neighbor(id hashType) []*node {
	t.RLock()
	bk := t.root.search(id)
	t.RUnlock()

	t.Lock()
	for _, node := range bk.clearTimeout() {
		delete(t.addrIndex, node.addr.String())
		node.close()
		t.size--
	}
	t.Unlock()
	return bk.getNodes()
}
