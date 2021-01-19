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

const discoveryCacheSize = 10000

// NodeMgr node manager
type NodeMgr struct {
	sync.RWMutex

	listen  *net.UDPConn
	id      [20]byte
	nodes   map[string]*Node // ip:port => node
	maxSize int

	// discovery
	disIdx   int
	disCache [discoveryCacheSize]string // tx
}

// NewNodeMgr new node manager
func NewNodeMgr(listen uint16, max int) (*NodeMgr, error) {
	c, err := net.ListenUDP("udp", &net.UDPAddr{Port: int(listen)})
	if err != nil {
		return nil, err
	}
	mgr := &NodeMgr{
		listen:  c,
		id:      data.RandID(),
		nodes:   make(map[string]*Node, max),
		maxSize: max,
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
				mgr.remove(node.HexID())
			}
		}
		time.Sleep(time.Second)
	}
}

func (mgr *NodeMgr) remove(id string) {
	mgr.Lock()
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
		logging.Error("node of %s not found", addr.String())
		return
	}
	var hdr data.Hdr
	err := bencode.Decode(buf, &hdr)
	if err != nil {
		logging.Error("decode data failed of %s, err=%v", node.HexID(), err)
		return
	}
	switch {
	case hdr.IsRequest():
		mgr.handleRequest(node, buf)
	case hdr.IsResponse():
		mgr.handleResponse(node, buf, hdr.Transaction)
	}
}

func (mgr *NodeMgr) handleRequest(node *Node, buf []byte) {
	switch data.ParseReqType(buf) {
	case data.TypePing:
		fmt.Println("handle ping")
	case data.TypeFindNode:
		fmt.Println("handle find_node")
	case data.TypeGetPeers:
		fmt.Println("handle get_peers")
	case data.TypeAnnouncePeer:
		fmt.Println("handle announce_peer")
	}
}

func (mgr *NodeMgr) handleResponse(node *Node, buf []byte, tx string) {
	for i := 0; i < discoveryCacheSize; i++ {
		if mgr.disCache[i] == tx {
			mgr.onDiscovery(node, buf)
			return
		}
	}
}

// Discovery discovery nodes
func (mgr *NodeMgr) Discovery(addrs []*net.UDPAddr) {
	mgr.bootstrap(addrs)
	maxSize := mgr.maxSize * 8 / 10
	for {
		if len(mgr.nodes) >= maxSize {
			time.Sleep(time.Second)
			continue
		}
		for _, node := range mgr.copyNodes() {
			mgr.sendDiscovery(node)
		}
		time.Sleep(time.Second)
	}
}

// http://www.bittorrent.org/beps/bep_0005.html
func (mgr *NodeMgr) sendDiscovery(node *Node) {
	data, tx, err := data.FindReq(mgr.id, data.RandID())
	if err != nil {
		logging.Error("build find_node packet failed of %s, err=%v", node.HexID(), err)
		return
	}
	_, err = mgr.listen.WriteTo(data, &node.addr)
	if err != nil {
		logging.Error("send find_node packet failed of %s, err=%v", node.HexID(), err)
		return
	}
	mgr.disCache[mgr.disIdx%discoveryCacheSize] = tx
	mgr.disIdx++
}

func (mgr *NodeMgr) bootstrap(addrs []*net.UDPAddr) {
	mgr.Lock()
	for _, addr := range addrs {
		node := newNode(data.RandID(), *addr)
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
		nextNode := newNode(next, addr)
		logging.Debug("discovery node %s, addr=%s", node.HexID(), node.addr.String())
		mgr.Lock()
		mgr.nodes[nextNode.HexID()] = nextNode
		mgr.Unlock()
		uniq[fmt.Sprintf("%x", next)] = true
	}
}
