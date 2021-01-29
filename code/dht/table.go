package dht

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lwch/hashmap"
	"github.com/lwch/magic/code/data"
	"github.com/lwch/magic/code/logging"
)

const nodeTimeout = 10 * time.Second

type addrData struct {
	addr string
	n    *node
}

type addrSlice struct {
	sync.Mutex
	data []addrData
	size int64
}

func (s *addrSlice) Make(size uint64) {
	s.data = make([]addrData, size)
}

func (s *addrSlice) Resize(size uint64) {
	s.Lock()
	defer s.Unlock()
	data := make([]addrData, size)
	copy(data, s.data)
	s.data = data
}

func (s *addrSlice) Size() uint64 {
	return uint64(s.size)
}

func (s *addrSlice) Cap() uint64 {
	return uint64(len(s.data))
}

func (s *addrSlice) Hash(key interface{}) uint64 {
	sum := md5.Sum([]byte(key.(string)))
	a := binary.BigEndian.Uint64(sum[:])
	b := binary.BigEndian.Uint64(sum[8:])
	return a + b
}

func (s *addrSlice) KeyEqual(idx uint64, key interface{}) bool {
	node := s.data[int(idx)%len(s.data)]
	return node.addr == key.(string)
}

func (s *addrSlice) Empty(idx uint64) bool {
	node := s.data[int(idx)%len(s.data)]
	return len(node.addr) == 0
}

func (s *addrSlice) Set(idx uint64, key, value interface{}, deadline time.Time, update bool) bool {
	target := &s.data[int(idx)%len(s.data)]
	target.addr = key.(string)
	target.n = value.(*node)
	if !update {
		atomic.AddInt64(&s.size, 1)
	}
	return true
}

func (s *addrSlice) Get(idx uint64) interface{} {
	node := s.data[int(idx)%len(s.data)]
	return node.n
}

func (s *addrSlice) Reset(idx uint64) {
	node := &s.data[int(idx)%len(s.data)]
	node.addr = ""
	node.n = nil
	atomic.AddInt64(&s.size, -1)
}

func (s *addrSlice) Timeout(idx uint64) bool {
	return false
}

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
		// TODO: update
		return false
	}
	if len(bk.nodes) >= k {
		loopSplit(bk, k)
		target := bk.search(n.id)
		if target.exists(n.id) {
			// TODO: update
			return false
		}
		target.nodes = append(target.nodes, n)
		return true
	}
	bk.nodes = append(bk.nodes, n)
	return true
}

func loopSplit(bk *bucket, k int) {
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

func (bk *bucket) clearTimeout() []*node {
	bk.Lock()
	defer bk.Unlock()
	removed := make([]*node, 0, len(bk.nodes))
	nodes := make([]*node, 0, len(bk.nodes))
	for _, node := range bk.nodes {
		since := time.Since(node.updated)
		if !node.isBootstrap && since >= nodeTimeout {
			logging.Debug("timeout: %s", node.id.String())
			node.close()
			removed = append(removed, node)
			continue
		} else if since >= nodeTimeout/2 {
			tx := node.sendPing()
			node.dht.tx.add(tx, data.TypePing, emptyHash, emptyHash)
		}
		nodes = append(nodes, node)
	}
	bk.nodes = nodes
	return removed
}

func newBucket(prefix hashType, bits int) *bucket {
	return &bucket{
		prefix: prefix,
		bits:   bits,
	}
}

type table struct {
	sync.RWMutex
	dht       *DHT
	root      *bucket
	addrIndex *hashmap.Map
	k         int
	size      int
	even      int
}

func newTable(dht *DHT, k int) *table {
	tb := &table{
		dht:       dht,
		root:      newBucket(emptyHash, 0),
		addrIndex: hashmap.New(&addrSlice{}, 1000, 5, 0),
		k:         k,
	}
	go func() {
		for {
			logging.Info("table: %d nodes", tb.size)
			time.Sleep(time.Second)
		}
	}()
	return tb
}

func (t *table) close() {
}

func (t *table) discoverySend(bk *bucket, limit *int) {
	if *limit <= 0 {
		return
	}
	if bk == nil {
		return
	}
	if bk.isLeaf() {
		for _, node := range bk.nodes {
			node.sendDiscovery()
		}
		*limit -= len(bk.nodes)
		for _, node := range bk.clearTimeout() {
			t.addrIndex.Remove(node.addr.String())
			t.size--
		}
		return
	}
	if t.even%2 == 0 {
		t.discoverySend(bk.leaf[0], limit)
		t.discoverySend(bk.leaf[1], limit)
	} else {
		t.discoverySend(bk.leaf[1], limit)
		t.discoverySend(bk.leaf[0], limit)
	}
	t.even++
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
				t.addrIndex.Set(n.addr.String(), n)
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
	for i, node := range bk.nodes {
		if !node.id.equal(n.id) {
			continue
		}
		bk.nodes = append(bk.nodes[:i], bk.nodes[i+1:]...)
		t.addrIndex.Remove(n.addr.String())
		t.size--
	}
}

func (t *table) findAddr(addr net.Addr) *node {
	t.RLock()
	defer t.RUnlock()
	data := t.addrIndex.Get(addr.String())
	if data == nil {
		return nil
	}
	n := data.(*node)
	// free node
	if t.even%2 == 0 {
		bk := t.root.search(n.id)
		for _, node := range bk.clearTimeout() {
			t.addrIndex.Remove(node.addr.String())
			t.size--
		}
		t.even++
	}
	return n
}

func (t *table) findID(id hashType) *node {
	t.RLock()
	defer t.RUnlock()
	bk := t.root.search(id)
	// free node
	defer func() {
		t.even++
		if t.even%2 == 0 {
			return
		}
		for _, node := range bk.clearTimeout() {
			t.addrIndex.Remove(node.addr.String())
			t.size--
		}
	}()
	for _, node := range bk.nodes {
		if node.id.equal(id) {
			return node
		}
	}
	return nil
}

func (t *table) neighbor(id hashType) []*node {
	t.RLock()
	defer t.RUnlock()
	bk := t.root.search(id)
	for _, node := range bk.clearTimeout() {
		t.addrIndex.Remove(node.addr.String())
		t.size--
	}
	return bk.nodes
}
