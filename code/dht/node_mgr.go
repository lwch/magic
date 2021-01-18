package dht

import (
	"sync"

	"github.com/lwch/magic/code/logging"
)

// NodeMgr node manager
type NodeMgr struct {
	sync.RWMutex
	id      [20]byte
	nodes   map[string]*Node
	maxSize int
}

// NewNodeMgr new node manager
func NewNodeMgr(myID [20]byte, max int) *NodeMgr {
	return &NodeMgr{
		id:      myID,
		nodes:   make(map[string]*Node, max),
		maxSize: max,
	}
}

// Exists node exists
func (m *NodeMgr) Exists(id string) bool {
	m.RLock()
	defer m.RUnlock()
	return m.nodes[id] != nil
}

// Push push node
func (m *NodeMgr) Push(node *Node) bool {
	if m.Exists(node.HexID()) {
		return false
	}
	if len(m.nodes) >= m.maxSize {
		logging.Info("full nodes")
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
	m.Lock()
	n := m.nodes[id]
	n.Close()
	delete(m.nodes, id)
	m.Unlock()
}
