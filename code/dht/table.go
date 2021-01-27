package dht

import (
	"context"
	"crypto/md5"
	"encoding/binary"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/lwch/hashmap"
	"github.com/lwch/magic/code/logging"
)

type tableNode struct {
	k string
	n *node
}

type tableSlice struct {
	sync.Mutex
	data []tableNode
	size int64
}

func (s *tableSlice) Make(size uint64) {
	s.data = make([]tableNode, size)
}

func (s *tableSlice) Resize(size uint64) {
	s.Lock()
	defer s.Unlock()
	data := make([]tableNode, size)
	copy(data, s.data)
	s.data = data
}

func (s *tableSlice) Size() uint64 {
	return uint64(s.size)
}

func (s *tableSlice) Cap() uint64 {
	return uint64(len(s.data))
}

func (s *tableSlice) Hash(key interface{}) uint64 {
	sum := md5.Sum([]byte(key.(string)))
	a := binary.BigEndian.Uint64(sum[:])
	b := binary.BigEndian.Uint64(sum[8:])
	return a + b
}

func (s *tableSlice) KeyEqual(idx uint64, key interface{}) bool {
	node := s.data[int(idx)%len(s.data)]
	return node.k == key.(string)
}

func (s *tableSlice) Empty(idx uint64) bool {
	node := s.data[int(idx)%len(s.data)]
	return len(node.k) == 0 && node.n == nil
}

func (s *tableSlice) Set(idx uint64, key, value interface{}, deadtime time.Time, update bool) bool {
	target := &s.data[int(idx)%len(s.data)]
	if len(target.k) > 0 || target.n != nil {
		return false
	}
	target.k = key.(string)
	target.n = value.(*node)
	if !update {
		s.size++
	}
	return true
}

func (s *tableSlice) Get(idx uint64) interface{} {
	node := s.data[int(idx)%len(s.data)]
	return node.n
}

func (s *tableSlice) Reset(idx uint64) {
	node := &s.data[int(idx)%len(s.data)]
	node.k = ""
	node.n = nil
	s.size--
}

func (s *tableSlice) Timeout(idx uint64) bool {
	return false
}

func (s *tableSlice) nodes() []*node {
	ret := make([]*node, 0, int(s.size))
	for i := uint64(0); i < s.Cap(); i++ {
		node := s.data[i]
		if len(node.k) > 0 {
			ret = append(ret, node.n)
		}
	}
	return ret
}

type table struct {
	dht            *DHT
	ipNodes        *hashmap.Map // addr => node
	idNodes        *hashmap.Map // id   => node
	max            int
	chDiscovery    chan *node
	bootstrapAddrs []*net.UDPAddr

	// runtime
	ctx    context.Context
	cancel context.CancelFunc
}

func newTable(dht *DHT, max int) *table {
	tb := &table{
		dht:         dht,
		ipNodes:     hashmap.New(&tableSlice{}, uint64(max), 5, 1, time.Second),
		idNodes:     hashmap.New(&tableSlice{}, uint64(max), 5, 1, time.Second),
		max:         max,
		chDiscovery: make(chan *node),
	}
	tb.ctx, tb.cancel = context.WithCancel(context.Background())
	go tb.keepalive()
	go tb.loopDiscovery()
	return tb
}

func (t *table) close() {
	t.cancel()
}

func (t *table) discovery() {
	run := func() {
		limit := (t.max - int(t.ipNodes.Size())) / 8
		for i, node := range t.copyNodes(t.ipNodes) {
			select {
			case t.chDiscovery <- node:
			case <-t.ctx.Done():
				return
			}
			if i >= limit {
				return
			}
		}
		limit = (t.max - int(t.idNodes.Size())) / 8
		for i, node := range t.copyNodes(t.idNodes) {
			select {
			case t.chDiscovery <- node:
			case <-t.ctx.Done():
				return
			}
			if i >= limit {
				return
			}
		}
	}
	for {
		if int(t.ipNodes.Size()) < t.max {
			run()
		} else if int(t.idNodes.Size()) < t.max {
			run()
		} else if t.dht.tx.size() == 0 {
			run()
		}
		select {
		case <-t.ctx.Done():
			return
		default:
			logging.Info("discovery: %d ip nodes, %d id nodes", t.ipNodes.Size(), t.idNodes.Size())
			time.Sleep(time.Second)
		}
	}
}

func (t *table) isFull() bool {
	return int(t.ipNodes.Size()) >= t.max ||
		int(t.idNodes.Size()) >= t.max
}

func (t *table) add(n *node) bool {
	if t.isFull() {
		return false
	}
	t.ipNodes.Set(n.addr.String(), n)
	t.idNodes.Set(n.id.String(), n)
	return true
}

func (t *table) keepalive() {
	for {
		select {
		case <-time.After(time.Second):
			t.checkKeepAlive()
		case <-t.ctx.Done():
			return
		}
	}
}

func (t *table) checkKeepAlive() {
	removed := make(map[*node]bool)
	check := func(list []*node) {
		for _, node := range list {
			sec := time.Since(node.updated).Seconds()
			if sec >= 10 {
				if !node.isBootstrap && !removed[node] {
					t.remove(node)
					removed[node] = true
					// t.dht.bl.blockID(node.id)
				}
			} else if sec >= 5 {
				node.sendPing(t.dht.listen, t.dht.local)
			}
		}
	}
	check(t.copyNodes(t.ipNodes))
	check(t.copyNodes(t.idNodes))
}

func (t *table) copyNodes(m *hashmap.Map) []*node {
	return m.Data().(*tableSlice).nodes()
}

func (t *table) remove(n *node) {
	n.close()
	t.ipNodes.Remove(n.addr.String())
	t.idNodes.Remove(n.id.String())
}

func (t *table) findAddr(addr net.Addr) *node {
	data := t.ipNodes.Get(addr.String())
	if data == nil {
		return nil
	}
	return data.(*node)
}

func (t *table) findID(id hashType) *node {
	data := t.idNodes.Get(id.String())
	if data == nil {
		return nil
	}
	return data.(*node)
}

func (t *table) loopDiscovery() {
	dt := time.Now()
	cnt := 0
	const limit = 100
	for {
		var n *node
		select {
		case n = <-t.chDiscovery:
		case <-t.ctx.Done():
			return
		}
		n.sendDiscovery(t.dht.listen, t.dht.local)
		now := time.Now()
		if dt.Unix() == now.Unix() {
			leftTime := 999999999 - now.Nanosecond()
			left := limit - cnt
			if left <= 0 {
				time.Sleep(time.Duration(leftTime))
			} else {
				time.Sleep(time.Duration(leftTime / left))
			}
		} else {
			dt = now
			cnt = 1
		}
	}
}

func (t *table) neighbor(id hashType, n int) []*node {
	nodes := t.copyNodes(t.idNodes)
	if len(nodes) < n {
		return nil
	}
	// random select
	ret := make([]*node, 0, n)
	for i := 0; i < n; i++ {
		ret = append(ret, nodes[rand.Intn(len(nodes))])
	}
	return ret
	// sort.Slice(nodes, func(i, j int) bool {
	// 	for x := 0; x < 20; x++ {
	// 		a := id[x] ^ nodes[i].id[x]
	// 		b := id[x] ^ nodes[j].id[x]
	// 		if a == b {
	// 			continue
	// 		}
	// 		return a < b
	// 	}
	// 	return false
	// })
	// return nodes[:n]
}
