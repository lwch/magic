package dht

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"time"

	"github.com/lwch/bencode"
	"github.com/lwch/magic/code/data"
	"github.com/lwch/magic/code/logging"
)

const maxFindCache = 100

// Node host
type Node struct {
	id      [20]byte
	c       *net.UDPConn
	chWrite chan []byte
	parent  *NodeMgr

	// control
	ctx    context.Context
	cancel context.CancelFunc

	// discovery
	findIdx int
	findTX  [maxFindCache]string
}

func newNode(parent *NodeMgr, id [20]byte, addr net.UDPAddr) (*Node, error) {
	c, err := net.DialUDP("udp", nil, &addr)
	if err != nil {
		return nil, err
	}
	node := &Node{
		id:      id,
		c:       c,
		chWrite: make(chan []byte),
		parent:  parent,
	}
	ctx, cancel := context.WithCancel(context.Background())
	node.ctx = ctx
	node.cancel = cancel
	go node.write()
	return node, nil
}

// ID get node id
func (n *Node) ID() [20]byte {
	return n.id
}

// HexID get node hex id
func (n *Node) HexID() string {
	return fmt.Sprintf("%x", n.id)
}

// C connection
func (n *Node) C() *net.UDPConn {
	return n.c
}

// Close close connection
func (n *Node) Close() {
	if n.c != nil {
		n.c.Close()
	}
	n.cancel()
	n.parent.Pop(n.HexID())
}

func (n *Node) write() {
	for {
		select {
		case data := <-n.chWrite:
			n.c.Write(data)
		case <-n.ctx.Done():
			return
		}
	}
}

// Work recv packet
func (n *Node) Work(id [20]byte) {
	defer n.Close()
	logging.Info("node %x work", n.id)
	go n.discovery(id)
	buf := make([]byte, 65535)
	for {
		len, err := n.c.Read(buf)
		if err != nil {
			logging.Error("read data of %s, err=%v", n.c.RemoteAddr().String(), err)
			return
		}
		var hdr data.Hdr
		err = bencode.Decode(buf[:len], &hdr)
		if err != nil {
			logging.Error("decode header of %s, err=%v\n%s", n.c.RemoteAddr().String(), err, hex.Dump(buf[:len]))
			return
		}
		switch {
		case hdr.IsRequest():
			n.handleRequest(buf[:len])
		case hdr.IsResponse():
			n.handleResponse(buf[:len], hdr)
		}
	}
}

func (n *Node) discovery(id [20]byte) {
	var next [20]byte
	for {
		select {
		case <-time.After(30 * time.Second):
			rand.Read(next[:])
			data, tx, err := data.FindReq(id, next)
			if err != nil {
				continue
			}
			n.findTX[n.findIdx%maxFindCache] = tx
			n.findIdx++
			n.chWrite <- data
		case <-n.ctx.Done():
			return
		}
	}
}

func (n *Node) handleRequest(buf []byte) {
	switch data.ParseReqType(buf) {
	case data.TypePing:
		fmt.Println("ping request")
	case data.TypeFindNode:
		fmt.Println("find_node request")
	}
}

func (n *Node) handleResponse(buf []byte, hdr data.Hdr) {
	for i := 0; i < maxFindCache; i++ {
		if n.findTX[i] == hdr.Transaction {
			n.handleDiscovery(buf)
			return
		}
	}
}

func (n *Node) handleDiscovery(buf []byte) {
	var findResp data.FindResponse
	err := bencode.Decode(buf, &findResp)
	if err != nil {
		return
	}
	uniq := make(map[string]bool)
	var next [20]byte
	for i := 0; i < len(findResp.Response.Nodes); i += 26 {
		var ip [4]byte
		var port uint16
		err = binary.Read(strings.NewReader(findResp.Response.Nodes[i+20:]), binary.BigEndian, &ip)
		if err != nil {
			continue
		}
		err = binary.Read(strings.NewReader(findResp.Response.Nodes[i+24:]), binary.BigEndian, &port)
		if err != nil {
			continue
		}
		copy(next[:], findResp.Response.Nodes[i:i+20])
		if uniq[fmt.Sprintf("%x", next)] {
			continue
		}
		node, err := newNode(n.parent, next, net.UDPAddr{
			IP:   net.IP(ip[:]),
			Port: int(port),
		})
		if err != nil {
			continue
		}
		logging.Info("discovery node %s, addr=%s", node.HexID(), node.C().RemoteAddr())
		if !n.parent.Push(node) {
			node.Close()
			continue
		}
		uniq[fmt.Sprintf("%x", next)] = true
	}
}
