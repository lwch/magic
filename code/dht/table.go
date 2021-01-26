package dht

import (
	"context"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/lwch/magic/code/data"
	"github.com/lwch/magic/code/logging"
)

type table struct {
	sync.RWMutex
	dht            *DHT
	ipNodes        map[string]node
	idNodes        map[string]node
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
		ipNodes:     make(map[string]node, max),
		idNodes:     make(map[string]node, max),
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

func (t *table) bootstrap(dht *DHT, addrs []*net.UDPAddr) []*node {
	t.bootstrapAddrs = addrs
	ret := make([]*node, 0, len(addrs))
	for _, addr := range addrs {
		node := newNode(dht, data.RandID(), *addr)
		ret = append(ret, node)
	}
	return ret
}

func (t *table) discovery() {
	run := func() {
		for _, node := range t.copyNodes(t.ipNodes) {
			select {
			case t.chDiscovery <- &node:
			case <-t.ctx.Done():
				return
			}
		}
		for _, node := range t.copyNodes(t.idNodes) {
			select {
			case t.chDiscovery <- &node:
			case <-t.ctx.Done():
				return
			}
		}
	}
	for {
		if len(t.ipNodes) == 0 || len(t.idNodes) == 0 {
			for _, node := range t.bootstrap(t.dht, t.bootstrapAddrs) {
				node.sendDiscovery(t.dht.listen, t.dht.local)
			}
			run()
		} else if len(t.ipNodes) < t.max/3 {
			run()
		} else if len(t.idNodes) < t.max/3 {
			run()
		} else if t.dht.tx.size() == 0 {
			run()
		}
		select {
		case <-t.ctx.Done():
			return
		default:
			logging.Info("discovery: %d ip nodes, %d id nodes", len(t.ipNodes), len(t.idNodes))
			time.Sleep(time.Second)
		}
	}
}

func (t *table) isFull() bool {
	return len(t.ipNodes) >= t.max ||
		len(t.idNodes) >= t.max
}

func (t *table) add(n *node) bool {
	if t.isFull() {
		return false
	}
	t.Lock()
	t.ipNodes[n.addr.String()] = *n
	t.idNodes[n.id.String()] = *n
	t.Unlock()
	return true
}

func (t *table) keepalive() {
	tk := time.NewTicker(time.Second)
	for {
		select {
		case <-tk.C:
			t.checkKeepAlive()
		case <-t.ctx.Done():
			return
		}
	}
}

func (t *table) checkKeepAlive() {
	check := func(list []node) {
		for _, node := range list {
			sec := time.Since(node.updated)
			if sec >= 10 {
				t.remove(node)
				// t.dht.bl.blockID(node.id)
			} else if sec >= 5 {
				node.sendPing(t.dht.listen, t.dht.local)
			}
		}
	}
	check(t.copyNodes(t.ipNodes))
	check(t.copyNodes(t.idNodes))
}

func (t *table) copyNodes(m map[string]node) []node {
	t.RLock()
	defer t.RUnlock()
	ret := make([]node, 0, len(m))
	for _, v := range m {
		ret = append(ret, v)
	}
	return ret
}

func (t *table) remove(n node) {
	n.close()
	t.Lock()
	defer t.Unlock()
	delete(t.ipNodes, n.addr.String())
	delete(t.idNodes, n.id.String())
}

func (t *table) findAddr(addr net.Addr) *node {
	t.RLock()
	defer t.RUnlock()
	if node, ok := t.ipNodes[addr.String()]; ok {
		return &node
	}
	return nil
}

func (t *table) findID(id hashType) *node {
	t.RLock()
	defer t.RUnlock()
	if node, ok := t.idNodes[id.String()]; ok {
		return &node
	}
	return nil
}

func (t *table) loopDiscovery() {
	for {
		var n *node
		select {
		case n = <-t.chDiscovery:
		case <-t.ctx.Done():
			return
		}
		n.sendDiscovery(t.dht.listen, t.dht.local)
	}
}

func (t *table) neighbor(id hashType, n int) []node {
	nodes := t.copyNodes(t.idNodes)
	if len(nodes) < n {
		return nil
	}
	// random select
	ret := make([]node, 0, n)
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
