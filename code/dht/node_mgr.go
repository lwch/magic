package dht

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/lwch/magic/code/data"
	"github.com/lwch/magic/code/logging"
)

const topSize = 8

// NodeMgr node manager
type NodeMgr struct {
	sync.RWMutex

	listen   *net.UDPConn
	id       [20]byte
	nodes    map[string]*Node // ip:port => node
	maxNodes int
	rm       *resMgr
}

// NewNodeMgr new node manager
func NewNodeMgr(listen uint16, maxNodes, maxRes, maxScan int) (*NodeMgr, error) {
	c, err := net.ListenUDP("udp", &net.UDPAddr{Port: int(listen)})
	if err != nil {
		return nil, err
	}
	mgr := &NodeMgr{
		listen:   c,
		id:       data.RandID(),
		nodes:    make(map[string]*Node, maxNodes),
		maxNodes: maxNodes,
		rm:       newResMgr(maxRes, maxScan),
	}
	go mgr.keepAlive()
	go mgr.recv()
	return mgr, nil
}

func (mgr *NodeMgr) copyNodes() []*Node {
	nodes := make([]*Node, 0, len(mgr.nodes))
	mgr.RLock()
	defer mgr.RUnlock()
	for _, node := range mgr.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

func (mgr *NodeMgr) keepAlive() {
	for {
		for _, node := range mgr.copyNodes() {
			if time.Since(node.updated).Seconds() >= 10 {
				mgr.remove(node.AddrString())
			}
		}
		logging.Info("%d nodes alive", len(mgr.nodes))
		time.Sleep(time.Second)
	}
}

func (mgr *NodeMgr) remove(id string) {
	mgr.Lock()
	if node := mgr.nodes[id]; node != nil {
		node.Close()
	}
	delete(mgr.nodes, id)
	mgr.Unlock()
}

func (mgr *NodeMgr) recv() {
	buf := make([]byte, 65535)
	for {
		n, addr, err := mgr.listen.ReadFrom(buf)
		if err != nil {
			continue
		}
		mgr.handleData(addr, buf[:n])
	}
}

func (mgr *NodeMgr) handleData(addr net.Addr, buf []byte) {
	mgr.RLock()
	node := mgr.nodes[addr.String()]
	mgr.RUnlock()
	if node == nil {
		// TODO: no log
		// logging.Error("node of %s not found", addr.String())
		return
	}
	data := make([]byte, len(buf))
	copy(data, buf)
	defer func() {
		recover()
	}()
	select {
	case node.chRead <- data:
	default:
	}
}

// Discovery discovery nodes
func (mgr *NodeMgr) Discovery(addrs []*net.UDPAddr) {
	mgr.bootstrap(addrs)
	for {
		if len(mgr.nodes) >= mgr.maxNodes {
			time.Sleep(time.Second)
			continue
		}
		nodes := mgr.copyNodes()
		rand.Shuffle(len(nodes), func(i, j int) {
			nodes[i], nodes[j] = nodes[j], nodes[i]
		})
		left := mgr.maxNodes - len(mgr.nodes)
		if left < 0 {
			time.Sleep(time.Second)
			continue
		}
		left /= 8 // each discovery response 8 nodes
		if left < len(mgr.nodes) {
			nodes = nodes[:left]
		}
		for _, node := range nodes {
			node.sendDiscovery(mgr.listen, mgr.id)
		}
		time.Sleep(time.Second)
	}
}

func (mgr *NodeMgr) bootstrap(addrs []*net.UDPAddr) {
	mgr.Lock()
	for _, addr := range addrs {
		node := newNode(mgr, mgr.id, data.RandID(), *addr)
		mgr.nodes[node.AddrString()] = node
	}
	mgr.Unlock()
}

// Exists node exists
func (mgr *NodeMgr) Exists(id [20]byte) bool {
	mgr.RLock()
	defer mgr.RUnlock()
	return mgr.nodes[fmt.Sprintf("%x", id)] != nil
}

func (mgr *NodeMgr) topK(id [20]byte, n int) []*Node {
	nodes := mgr.copyNodes()
	if len(nodes) < n {
		return nil
	}
	sort.Slice(nodes, func(i, j int) bool {
		for x := 0; x < 20; x++ {
			a := id[x] ^ nodes[i].id[x]
			b := id[x] ^ nodes[j].id[x]
			if a == b {
				continue
			}
			return a < b
		}
		return false
	})
	return nodes[:n]
}

func formatNodes(nodes []*Node) []byte {
	ret := make([]byte, len(nodes)*26)
	for i := 0; i < len(nodes); i++ {
		node := nodes[i]
		copy(ret[i*26:], node.id[:])
		var ipPort bytes.Buffer
		binary.Write(&ipPort, binary.BigEndian, node.addr.IP)
		binary.Write(&ipPort, binary.BigEndian, uint16(node.addr.Port))
		copy(ret[i*26+20:], ipPort.Bytes())
	}
	return ret
}
