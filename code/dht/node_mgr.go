package dht

import (
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/lwch/bencode"
	"github.com/lwch/magic/code/data"
	"github.com/lwch/magic/code/logging"
)

// NodeMgr node manager
type NodeMgr struct {
	sync.RWMutex
	id       [20]byte
	nodes    map[string]*Node // id => node
	maxSize  int
	chRemove chan string
}

// NewNodeMgr new node manager
func NewNodeMgr(myID [20]byte, max int) *NodeMgr {
	ret := &NodeMgr{
		id:       myID,
		nodes:    make(map[string]*Node, max),
		maxSize:  max,
		chRemove: make(chan string, 100),
	}
	go ret.keepAlive()
	return ret
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
				mgr.remove(node.HexID())
			}
		}
		time.Sleep(time.Second)
	}
}

func (mgr *NodeMgr) remove(id string) {
	mgr.RLock()
	node := mgr.nodes[id]
	mgr.RUnlock()

	if node == nil {
		return
	}
	node.Close()

	mgr.Lock()
	delete(mgr.nodes, id)
	mgr.Unlock()
}

// Discovery discovery nodes
func (mgr *NodeMgr) Discovery(addrs []*net.UDPAddr) {
	mgr.bootstrap(addrs)
	for {
		if len(mgr.nodes) >= mgr.maxSize {
			time.Sleep(time.Second)
			continue
		}
		for _, node := range mgr.copyNodes() {
			node.sendDiscovery(mgr.id)
		}
		time.Sleep(time.Second)
	}
}

func (mgr *NodeMgr) bootstrap(addrs []*net.UDPAddr) {
	mgr.Lock()
	var id [20]byte
	for _, addr := range addrs {
		copy(id[:], data.Rand(20))
		node, err := newNode(mgr, id, addr)
		if err != nil {
			continue
		}
		mgr.nodes[node.HexID()] = node
	}
	mgr.Unlock()
}

func (mgr *NodeMgr) onDiscovery(node *Node, buf []byte) {
	var resp data.FindResponse
	err := bencode.Decode(buf, &resp)
	if err != nil {
		logging.Error("decode discovery data failed of %s, err=%v", node.HexID(), err)
		return
	}
	uniq := make(map[string]bool)
	for i := 0; i < len(resp.Response.Nodes); i += 26 {
		if len(mgr.nodes) >= mgr.maxSize {
			logging.Info("full nodes")
			return
		}
		var ip [4]byte
		var port uint16
		err = binary.Read(strings.NewReader(resp.Response.Nodes[i+20:]), binary.BigEndian, &ip)
		if err != nil {
			logging.Error("read ip failed of %s, err=%v", node.HexID(), err)
			continue
		}
		err = binary.Read(strings.NewReader(resp.Response.Nodes[i+24:]), binary.BigEndian, &port)
		if err != nil {
			logging.Error("read port failed of %s, err=%v", node.HexID(), err)
			continue
		}
		var next [20]byte
		copy(next[:], resp.Response.Nodes[i:i+20])
		if uniq[fmt.Sprintf("%x", next)] {
			continue
		}
		addr := net.UDPAddr{
			IP:   net.IP(ip[:]),
			Port: int(port),
		}
		nextNode, err := newNode(mgr, next, &addr)
		if err != nil {
			logging.Error("create node of %s failed, addr=%s, err=%v", node.HexID(), addr.String(), err)
			continue
		}
		logging.Info("discovery node %s, addr=%s", node.HexID(), node.c.RemoteAddr())
		mgr.Lock()
		if n := mgr.nodes[nextNode.HexID()]; n != nil {
			n.Close()
		}
		mgr.nodes[nextNode.HexID()] = nextNode
		mgr.Unlock()
		uniq[fmt.Sprintf("%x", next)] = true
	}
}
