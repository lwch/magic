package dht

import (
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"time"

	"github.com/lwch/bencode"
	"github.com/lwch/magic/code/data"
	"github.com/lwch/magic/code/logging"
)

const discoveryCacheSize = 4 // cache size of find_node tx
const getCacheSize = 4       // cache size of get_peers tx
const getLogCacheSize = 10   // cache size of get_log

type getLog struct {
	hash [20]byte
	tx   string
}

// Node host
type Node struct {
	parent  *NodeMgr
	id      [20]byte    // remote id
	addr    net.UDPAddr // remote addr
	updated time.Time
	chRead  chan []byte

	// context
	ctx    context.Context
	cancel context.CancelFunc

	// discovery
	disIdx   int
	disCache [discoveryCacheSize]string // tx

	// scan
	getLogIdx   int
	getLogCache [getLogCacheSize]getLog // tx => hash
	getIdx      int
	getCache    [getCacheSize]string // tx
}

func newNode(parent *NodeMgr, localID, id [20]byte, addr net.UDPAddr) *Node {
	ctx, cancel := context.WithCancel(context.Background())
	node := &Node{
		parent:  parent,
		id:      id,
		addr:    addr,
		updated: time.Now(),
		chRead:  make(chan []byte, 2),

		ctx:    ctx,
		cancel: cancel,
	}
	// go node.keepAlive(localID)
	go node.recv()
	return node
}

// ID get node id
func (node *Node) ID() [20]byte {
	return node.id
}

// HexID get node hex id
func (node *Node) HexID() string {
	return fmt.Sprintf("%x", node.id)
}

// AddrString string of address
func (node *Node) AddrString() string {
	return node.addr.String()
}

// Close close node
func (node *Node) Close() {
	node.cancel()
	// close(node.chRead) // TODO: debug close many times
}

// http://www.bittorrent.org/beps/bep_0005.html
func (node *Node) sendDiscovery(c *net.UDPConn, id [20]byte) {
	var next [20]byte
	rand.Read(next[:])
	data, tx, err := data.FindReq(id, next)
	if err != nil {
		logging.Error("build find_node packet failed of %s, err=%v", node.AddrString(), err)
		return
	}
	_, err = c.WriteTo(data, &node.addr)
	if err != nil {
		logging.Error("send find_node packet failed of %s, err=%v", node.AddrString(), err)
		return
	}
	node.disCache[node.disIdx%discoveryCacheSize] = tx
	node.disIdx++
}

func (node *Node) sendGet(c *net.UDPConn, id, hash [20]byte) {
	data, tx, err := data.GetPeers(id, hash)
	if err != nil {
		logging.Error("build get_peers packet failed of %s, err=%v", node.AddrString(), err)
		return
	}
	node.getLogCache[node.getLogIdx%getLogCacheSize] = getLog{
		hash: hash,
		tx:   tx,
	}
	node.getLogIdx++
	node.getCache[node.getIdx%getCacheSize] = tx
	node.getIdx++
	_, err = c.WriteTo(data, &node.addr)
	if err != nil {
		logging.Error("send get_peers packet failed of %s, err=%v", node.AddrString(), err)
		return
	}
}

func (node *Node) keepAlive(id [20]byte) {
	for {
		select {
		case <-time.After(5 * time.Second):
			req, _, err := data.PingReq(id)
			if err != nil {
				logging.Error("build ping request of %s failed, err=%v", node.HexID(), err)
				return
			}
			_, err = node.parent.listen.WriteTo(req, &node.addr)
			if err != nil {
				logging.Error("send ping request of %s failed, err=%v", node.HexID(), err)
				continue
			}
		case <-node.ctx.Done():
			return
		}
	}
}

func (node *Node) recv() {
	for {
		select {
		case data := <-node.chRead:
			node.onData(data)
		case <-node.ctx.Done():
			return
		}
	}
}

func (node *Node) onData(buf []byte) {
	node.updated = time.Now()
	var hdr data.Hdr
	err := bencode.Decode(buf, &hdr)
	if err != nil {
		// invalid data means wrong client
		// logging.Error("decode data failed of %s, err=%v\n%s", node.HexID(), err, hex.Dump(buf))
		return
	}
	switch {
	case hdr.IsRequest():
		node.handleRequest(buf)
	case hdr.IsResponse():
		node.handleResponse(buf, hdr.Transaction)
	}
}

func (node *Node) handleRequest(buf []byte) {
	switch data.ParseReqType(buf) {
	case data.TypePing:
		node.parent.onPing(node, buf)
	case data.TypeFindNode:
		node.parent.onFindNode(node, buf)
	case data.TypeGetPeers:
		node.parent.onGetPeers(node, buf)
	case data.TypeAnnouncePeer:
		node.parent.onAnnouncePeer(node, buf)
	}
}

func (node *Node) handleResponse(buf []byte, tx string) {
	for i := 0; i < discoveryCacheSize; i++ {
		if node.disCache[i] == tx {
			node.parent.onDiscovery(node, buf)
			return
		}
	}
	for i := 0; i < getCacheSize; i++ {
		if node.getCache[i] == tx {
			for j := 0; j < getLogCacheSize; j++ {
				log := node.getLogCache[j]
				if log.tx == tx {
					node.parent.onGetPeersResponse(node, buf, log.hash)
					return
				}
			}
			return
		}
	}
}
