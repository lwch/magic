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

type pingPkt struct {
	buf  []byte
	addr net.UDPAddr
}

type node struct {
	dht         *DHT
	id          hashType
	addr        net.UDPAddr
	updated     time.Time
	chPing      chan pingPkt
	chPong      chan struct{}
	chClose     chan struct{}
	isBootstrap bool
}

func newNode(dht *DHT, id hashType, addr net.UDPAddr) *node {
	n := dht.nodePool.Get().(*node)
	n.id = id
	n.addr = addr
	n.updated = time.Now()
	n.chPong = make(chan struct{}, 10)
	n.chClose = make(chan struct{})
	return n
}

func newBootstrapNode(dht *DHT, addr net.UDPAddr) *node {
	n := dht.nodePool.Get().(*node)
	n.id = data.RandID()
	n.addr = addr
	n.updated = time.Now()
	n.isBootstrap = true
	return n
}

func (n *node) close() {
	n.chClose <- struct{}{}
	n.dht.nodePool.Put(n)
}

// http://www.bittorrent.org/beps/bep_0005.html
func (n *node) sendDiscovery() {
	var next [20]byte
	rand.Read(next[:])
	pkt, tx, err := data.FindReq(n.dht.local, next)
	if err != nil {
		logging.Error("build find_node packet failed" + n.errInfo(err))
		return
	}
	_, err = n.dht.listen.WriteTo(pkt, &n.addr)
	if err != nil {
		logging.Error("send find_node packet failed" + n.errInfo(err))
		return
	}
	n.dht.tx.add(tx, data.TypeFindNode, emptyHash, n.id)
}

func (n *node) sendPing() string {
	buf, tx, err := data.PingReq(n.dht.local)
	if err != nil {
		logging.Error("build get_peers packet failed" + n.errInfo(err))
		return ""
	}
	select {
	case n.chPing <- pingPkt{
		buf:  buf,
		addr: n.addr,
	}:
		return tx
	case <-time.After(time.Second):
		logging.Error("busy ping" + n.info())
		return ""
	}
}

func (n *node) loopPing() {
	for {
		select {
		case pkt := <-n.chPing:
			n.dht.listen.WriteTo(pkt.buf, &pkt.addr)
		case <-time.After(10 * time.Second):
		case <-n.chClose:
		}
	}
}

func (n *node) sendGet(hash hashType) {
	buf, tx, err := data.GetPeers(n.dht.local, hash)
	if err != nil {
		logging.Error("build get_peers packet failed" + n.errInfo(err))
		return
	}
	_, err = n.dht.listen.WriteTo(buf, &n.addr)
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
	err := bencode.Decode(buf, &req)
	if err != nil {
		logging.Error("decode request failed" + n.errInfo(err))
		return
	}
	if !n.id.equal(req.Data.ID) {
		n.dht.tb.remove(n)
		return
	}
	switch data.ParseReqType(buf) {
	case data.TypePing:
		n.onPing(buf)
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

func (n *node) info() string {
	return fmt.Sprintf("; id=%s, addr=%s",
		n.id.String(), n.addr.String())
}
