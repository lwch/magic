package dht

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/lwch/bencode"
	"github.com/lwch/magic/code/data"
	"github.com/lwch/magic/code/logging"
)

const topSize = 8

// NodeMgr node manager
type NodeMgr struct {
	sync.RWMutex

	listen  *net.UDPConn
	id      [20]byte
	nodes   map[string]*Node // ip:port => node
	maxSize int
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
		if len(mgr.nodes) >= mgr.maxSize {
			time.Sleep(time.Second)
			continue
		}
		nodes := mgr.copyNodes()
		rand.Shuffle(len(nodes), func(i, j int) {
			nodes[i], nodes[j] = nodes[j], nodes[i]
		})
		left := mgr.maxSize - len(mgr.nodes)
		if left < 0 {
			time.Sleep(time.Second)
			continue
		}
		left /= 8
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
		nextNode := newNode(mgr, mgr.id, next, addr)
		logging.Debug("discovery node %s, addr=%s", node.HexID(), node.AddrString())
		mgr.Lock()
		if node := mgr.nodes[nextNode.AddrString()]; node != nil {
			node.Close()
		}
		mgr.nodes[nextNode.AddrString()] = nextNode
		mgr.Unlock()
		uniq[fmt.Sprintf("%x", next)] = true
	}
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

func (mgr *NodeMgr) onPing(node *Node, buf []byte) {
	data, err := data.PingRep(mgr.id)
	if err != nil {
		logging.Error("build ping response packet failed of %s, err=%v", node.HexID(), err)
		return
	}
	_, err = mgr.listen.WriteTo(data, &node.addr)
	if err != nil {
		logging.Error("send ping response packet failed of %s, err=%v", node.HexID(), err)
		return
	}
}

func (mgr *NodeMgr) onFindNode(node *Node, buf []byte) {
	var req data.FindRequest
	err := bencode.Decode(buf, &req)
	if err != nil {
		logging.Error("parse find_node packet failed of %s, err=%v", node.HexID(), err)
		return
	}
	nodes := mgr.topK(req.Data.Target, topSize)
	if nodes == nil {
		logging.Info("less nodes")
		return
	}
	data, err := data.FindRep(mgr.id, string(formatNodes(nodes)))
	if err != nil {
		logging.Error("build find_node response packet failed of %s, err=%v", node.HexID(), err)
		return
	}
	_, err = mgr.listen.WriteTo(data, &node.addr)
	if err != nil {
		logging.Error("send find_node response packet failed of %s, err=%v", node.HexID(), err)
		return
	}
}

func (mgr *NodeMgr) onGetPeers(node *Node, buf []byte) {
	var req data.GetPeersRequest
	err := bencode.Decode(buf, &req)
	if err != nil {
		logging.Error("parse get_peers packet failed of %s, err=%v", node.HexID(), err)
		return
	}
	logging.Info("get_peers: %x", req.Data.Hash)
	nodes := mgr.topK(req.Data.Hash, topSize)
	if nodes == nil {
		logging.Info("less nodes")
		return
	}
	data, err := data.GetPeersNotFound(mgr.id, data.Rand(32), string(formatNodes(nodes)))
	if err != nil {
		logging.Error("build get_peers response packet failed of %s, err=%v", node.HexID(), err)
		return
	}
	_, err = mgr.listen.WriteTo(data, &node.addr)
	if err != nil {
		logging.Error("send get_peers response packet failed of %s, err=%v", node.HexID(), err)
		return
	}
}

func (mgr *NodeMgr) onAnnouncePeer(node *Node, buf []byte) {
	var req data.AnnouncePeerRequest
	err := bencode.Decode(buf, &req)
	if err != nil {
		logging.Error("parse announce_peer packet failed of %s, err=%v", node.HexID(), err)
		return
	}
	logging.Info("announce_peer: %x", req.Data.Hash)
	data, err := data.AnnouncePeer(mgr.id)
	if err != nil {
		logging.Error("build announce_peer response packet failed of %s, err=%v", node.HexID(), err)
		return
	}
	_, err = mgr.listen.WriteTo(data, &node.addr)
	if err != nil {
		logging.Error("send announce_peer response packet failed of %s, err=%v", node.HexID(), err)
		return
	}
}
