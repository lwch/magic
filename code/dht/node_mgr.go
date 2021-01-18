package dht

import (
	"sync"
)

// NodeMgr node manager
type NodeMgr struct {
	sync.RWMutex
	id       [20]byte
	nodes    map[string]*Node
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
	go ret.clear()
	return ret
}

// Exists node exists
func (m *NodeMgr) Exists(id string) bool {
	m.RLock()
	defer m.RUnlock()
	return m.nodes[id] != nil
}

// isFull
func (m *NodeMgr) isFull() bool {
	return len(m.nodes) >= m.maxSize
}

// Push push node
func (m *NodeMgr) Push(node *Node) bool {
	if m.Exists(node.HexID()) {
		return false
	}
	if len(m.nodes) >= m.maxSize {
		return false
	}
	go node.Work(m.id)
	m.Lock()
	m.nodes[node.HexID()] = node
	m.Unlock()
	return true
}

// Pop pop node
func (m *NodeMgr) Pop(id string) {
	if !m.Exists(id) {
		return
	}
	m.chRemove <- id
}

func (m *NodeMgr) clear() {
	for {
		id := <-m.chRemove
		m.Lock()
		delete(m.nodes, id)
		m.Unlock()
	}
}
