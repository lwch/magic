package dht

import (
	"crypto/rand"
	"net"
	"time"

	"github.com/lwch/magic/code/data"
	"github.com/lwch/magic/code/logging"
)

type node struct {
	dht     *DHT
	id      idType
	addr    net.UDPAddr
	updated time.Time
}

func newNode(dht *DHT, id idType, addr net.UDPAddr) *node {
	return &node{
		dht:  dht,
		id:   id,
		addr: addr,
	}
}

func (n *node) close() {

}

// http://www.bittorrent.org/beps/bep_0005.html
func (n *node) sendDiscovery(c *net.UDPConn, id idType) {
	var next [20]byte
	rand.Read(next[:])
	data, tx, err := data.FindReq(id, next)
	if err != nil {
		logging.Error("build find_node packet failed of %s, err=%v", n.addr.String(), err)
		return
	}
	_, err = c.WriteTo(data, &n.addr)
	if err != nil {
		logging.Error("send find_node packet failed of %s, err=%v", n.addr.String(), err)
		return
	}
	n.dht.tx.add(tx, "find_node", emptyHash, n.id)
}

func (n *node) onRecv(buf []byte) {
	n.updated = time.Now()
}
