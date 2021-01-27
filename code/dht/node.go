package dht

import (
	"crypto/rand"
	"fmt"
	"net"
	"time"

	"github.com/lwch/bencode"
	"github.com/lwch/magic/code/data"
	"github.com/lwch/magic/code/logging"
)

type node struct {
	dht         *DHT
	id          hashType
	addr        net.UDPAddr
	updated     time.Time
	chPong      chan struct{}
	isBootstrap bool
}

func newNode(dht *DHT, id hashType, addr net.UDPAddr) *node {
	return &node{
		dht:     dht,
		id:      id,
		addr:    addr,
		updated: time.Now(),
		chPong:  make(chan struct{}),
	}
}

func newBootstrapNode(dht *DHT, addr net.UDPAddr) *node {
	return &node{
		dht:         dht,
		id:          data.RandID(),
		addr:        addr,
		updated:     time.Now(),
		isBootstrap: true,
	}
}

func (n *node) close() {
}

// http://www.bittorrent.org/beps/bep_0005.html
func (n *node) sendDiscovery(c *net.UDPConn, id hashType) {
	var next [20]byte
	rand.Read(next[:])
	pkt, tx, err := data.FindReq(id, next)
	if err != nil {
		logging.Error("build find_node packet failed" + n.errInfo(err))
		return
	}
	_, err = c.WriteTo(pkt, &n.addr)
	if err != nil {
		logging.Error("send find_node packet failed" + n.errInfo(err))
		return
	}
	n.dht.tx.add(tx, data.TypeFindNode, emptyHash, n.id)
}

func (n *node) sendPing(c *net.UDPConn, local hashType) string {
	buf, tx, err := data.PingReq(local)
	if err != nil {
		logging.Error("build get_peers packet failed" + n.errInfo(err))
		return ""
	}
	_, err = c.WriteTo(buf, &n.addr)
	if err != nil {
		logging.Error("send get_peers packet failed" + n.errInfo(err))
		return ""
	}
	n.dht.tx.add(tx, data.TypePing, emptyHash, emptyHash)
	return tx
}

func (n *node) sendGet(c *net.UDPConn, local, hash hashType) {
	buf, tx, err := data.GetPeers(local, hash)
	if err != nil {
		logging.Error("build get_peers packet failed" + n.errInfo(err))
		return
	}
	_, err = c.WriteTo(buf, &n.addr)
	if err != nil {
		logging.Error("send get_peers packet failed" + n.errInfo(err))
		return
	}
	n.dht.tx.add(tx, data.TypeGetPeers, hash, emptyHash)
}

func (n *node) onRecv(buf []byte) {
	n.updated = time.Now()
	var hdr data.Hdr
	err := bencode.Decode(buf, &hdr)
	if err != nil {
		// TODO: log
		return
	}
	switch {
	case hdr.IsRequest():
		n.handleRequest(buf)
	case hdr.IsResponse():
		n.handleResponse(buf, hdr.Transaction)
	}
}

func (n *node) handleRequest(buf []byte) {
	var req struct {
		Data struct {
			ID [20]byte `bencode:"id"`
		} `bencode:"a"`
	}
	bencode.Decode(buf, &req)
	if !n.id.equal(req.Data.ID) {
		// n.dht.bl.blockAddr(&n.addr)
		// n.dht.bl.blockID(n.id)
		n.dht.tb.remove(n)
		return
	}
	switch data.ParseReqType(buf) {
	case data.TypePing:
		n.onPing()
	case data.TypeFindNode:
		n.onFindNode(buf)
	case data.TypeGetPeers:
		n.onGetPeers(buf)
	case data.TypeAnnouncePeer:
		n.onAnnouncePeer(buf)
	}
}

func (n *node) handleResponse(buf []byte, tx string) {
	txr := n.dht.tx.find(tx)
	if txr == nil {
		return
	}
	switch txr.t {
	case data.TypePing:
		n.updated = time.Now()
		select {
		case n.chPong <- struct{}{}:
		default:
		}
	case data.TypeFindNode:
		n.onFindNodeResp(buf)
	case data.TypeGetPeers:
		n.onGetPeersResp(buf, txr.hash)
	}
}

func (n *node) errInfo(err error) string {
	return fmt.Sprintf("; id=%s, addr=%s, err=%v",
		n.id.String(), n.addr.String(), err)
}
