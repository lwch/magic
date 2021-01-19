package dht

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"time"

	"github.com/lwch/bencode"
	"github.com/lwch/magic/code/data"
	"github.com/lwch/magic/code/logging"
)

const discoveryCacheSize = 100

// Node host
type Node struct {
	parent  *NodeMgr
	local   [20]byte // random local id
	id      [20]byte // remote id
	updated time.Time
	c       *net.UDPConn

	// control
	ctx    context.Context
	cancel context.CancelFunc

	// discovery
	disIdx   int
	disCache [discoveryCacheSize]string // tx
}

func newNode(parent *NodeMgr, id [20]byte, addr *net.UDPAddr) (*Node, error) {
	c, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	var local [20]byte
	rand.Read(local[:])
	node := &Node{
		parent:  parent,
		local:   local,
		id:      id,
		updated: time.Now(),
		c:       c,
		ctx:     ctx,
		cancel:  cancel,
	}
	go node.recv()
	return node, nil
}

// ID get node id
func (node *Node) ID() [20]byte {
	return node.id
}

// HexID get node hex id
func (node *Node) HexID() string {
	return fmt.Sprintf("%x", node.id)
}

// Close close node
func (node *Node) Close() {
	node.cancel()
	if node.c != nil {
		node.c.Close()
	}
}

// http://www.bittorrent.org/beps/bep_0005.html
func (node *Node) sendDiscovery() {
	var next [20]byte
	rand.Read(next[:])
	data, tx, err := data.FindReq(node.local, next)
	if err != nil {
		logging.Error("build find_node packet failed of %s, err=%v", node.HexID(), err)
		return
	}
	_, err = node.c.Write(data)
	if err != nil {
		logging.Error("send find_node packet failed of %s, err=%v", node.HexID(), err)
		return
	}
	node.disCache[node.disIdx%discoveryCacheSize] = tx
	node.disIdx++
}

func (node *Node) recv() {
	buf := make([]byte, 65535)
	for {
		select {
		case <-node.ctx.Done():
			return
		default:
		}

		node.c.SetReadDeadline(time.Now().Add(time.Second))
		n, err := node.c.Read(buf)
		if err != nil {
			continue
		}
		node.updated = time.Now()
		node.handleData(buf[:n])
	}
}

func (node *Node) handleData(buf []byte) {
	var hdr data.Hdr
	err := bencode.Decode(buf, &hdr)
	if err != nil {
		logging.Error("decode data failed of %s, err=%v", node.HexID(), err)
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
		fmt.Println("handle ping")
	case data.TypeFindNode:
		fmt.Println("handle find_node")
	case data.TypeGetPeers:
		fmt.Println("handle get_peers")
	case data.TypeAnnouncePeer:
		fmt.Println("handle announce_peer")
	}
}

func (node *Node) handleResponse(buf []byte, tx string) {
	for i := 0; i < discoveryCacheSize; i++ {
		if node.disCache[i] == tx {
			node.parent.onDiscovery(node, buf)
			return
		}
	}
}
