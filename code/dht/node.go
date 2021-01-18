package dht

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"time"

	"github.com/lwch/bencode"
	"github.com/lwch/magic/code/data"
	"github.com/lwch/magic/code/logging"
)

// Node host
type Node struct {
	id      [20]byte
	c       *net.UDPConn
	chWrite chan []byte

	ctx    context.Context
	cancel context.CancelFunc
}

func newNode(id [20]byte, addr net.UDPAddr) (*Node, error) {
	c, err := net.DialUDP("udp", nil, &addr)
	if err != nil {
		return nil, err
	}
	node := &Node{
		id:      id,
		c:       c,
		chWrite: make(chan []byte),
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
	defer n.c.Close()
	logging.Info("node %x work", n.id)
	go n.keepAlive(id)
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
			n.handleResponse(buf[:len])
		}
	}
}

func (n *Node) keepAlive(id [20]byte) {
	req, _ := data.PingReq(id)
	for {
		select {
		case <-time.After(30 * time.Second):
			n.chWrite <- req
		case <-n.ctx.Done():
			return
		}
	}
}

func (n *Node) handleRequest(buf []byte) {
	switch data.ParseReqType(buf) {
	case data.TypePing:
		fmt.Println("ping")
	case data.TypeFindNode:
		fmt.Println("find_node")
	}
}

func (n *Node) handleResponse(buf []byte) {

}
