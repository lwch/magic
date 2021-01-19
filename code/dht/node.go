package dht

import (
	"net"
	"time"
)

// Node host
type Node struct {
	id      [20]byte    // remote id
	addr    net.UDPAddr // remote addr
	updated time.Time
}

func newNode(id [20]byte, addr net.UDPAddr) *Node {
	return &Node{
		id:      id,
		addr:    addr,
		updated: time.Now(),
	}
}

// ID get node id
func (node *Node) ID() [20]byte {
	return node.id
}

// HexID get node hex id
func (node *Node) HexID() string {
	return node.addr.String()
}
