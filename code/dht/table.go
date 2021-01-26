package dht

import (
	"context"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/lwch/magic/code/logging"
)

type table struct {
	sync.RWMutex
	dht     *DHT
	ipNodes map[string]node
	idNodes map[string]node
	max     int

	// runtime
	ctx    context.Context
	cancel context.CancelFunc
}

func newTable(dht *DHT, max int) *table {
	tb := &table{
		dht:     dht,
		ipNodes: make(map[string]node, max),
		idNodes: make(map[string]node, max),
		max:     max,
	}
	tb.ctx, tb.cancel = context.WithCancel(context.Background())
	go tb.keepalive()
	return tb
}

func (t *table) close() {
	t.cancel()
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
	for _, node := range t.copyNodes(t.ipNodes) {
		if time.Since(node.updated).Seconds() >= 10 {
			t.remove(node)
		}
	}
	for _, node := range t.copyNodes(t.idNodes) {
		if time.Since(node.updated).Seconds() >= 10 {
			t.remove(node)
		}
	}
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

func (t *table) onDiscovery(c *net.UDPConn) {
	run := func(m map[string]node) {
		left := t.max - len(m)
		maxLimit := left / 8
		if maxLimit <= 0 {
			return
		}
		nodes := t.copyNodes(m)
		if maxLimit > len(nodes) {
			maxLimit = len(nodes)
		} else {
			rand.Shuffle(len(nodes), func(i, j int) {
				nodes[i], nodes[j] = nodes[j], nodes[i]
			})
			nodes = nodes[:maxLimit]
		}
		for _, node := range nodes {
			node.sendDiscovery(c, t.dht.local)
		}
	}
	run(t.ipNodes)
	run(t.idNodes)
	logging.Info("discovery: %d ip nodes, %d id nodes", len(t.ipNodes), len(t.idNodes))
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
