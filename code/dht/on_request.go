package dht

import (
	"bytes"
	"encoding/binary"
	"net"

	"github.com/lwch/bencode"
	"github.com/lwch/magic/code/data"
	"github.com/lwch/magic/code/logging"
)

func (n *node) onPing() {
	data, err := data.PingRep(n.dht.local)
	if err != nil {
		logging.Error("build ping response packet failed" + n.errInfo(err))
		return
	}
	_, err = n.dht.listen.WriteTo(data, &n.addr)
	if err != nil {
		logging.Error("send ping response packet failed" + n.errInfo(err))
		return
	}
}

func compactNodes(nodes []*node) []byte {
	ret := make([]byte, len(nodes)*26)
	for i := 0; i < len(nodes); i++ {
		node := nodes[i]
		copy(ret[i*26:], node.id[:])
		var ipPort bytes.Buffer
		binary.Write(&ipPort, binary.BigEndian, node.addr.IP)
		binary.Write(&ipPort, binary.BigEndian, uint16(node.addr.Port))
		copy(ret[i*26+20:], ipPort.Bytes())
	}
	return ret
}

func (n *node) onFindNode(buf []byte) {
	var req data.FindRequest
	err := bencode.Decode(buf, &req)
	if err != nil {
		logging.Error("decode find_node request failed" + n.errInfo(err))
		return
	}
	nodes := n.dht.tb.neighbor(req.Data.Target)
	data, err := data.FindRep(n.dht.local, string(compactNodes(nodes)))
	if err != nil {
		logging.Error("build find_node response packet faield" + n.errInfo(err))
		return
	}
	_, err = n.dht.listen.WriteTo(data, &n.addr)
	if err != nil {
		logging.Error("send find_node response packet failed" + n.errInfo(err))
		return
	}
}

func (n *node) onGetPeers(buf []byte) {
	var req data.GetPeersRequest
	err := bencode.Decode(buf, &req)
	if err != nil {
		logging.Error("decode get_peers request failed" + n.errInfo(err))
		return
	}
	// logging.Info("get_peers: %x", req.Data.Hash)
	nodes := n.dht.tb.neighbor(req.Data.Hash)
	if len(nodes) == 0 {
		logging.Error("not enough nodes")
		return
	}
	data, err := data.GetPeersNotFound(n.dht.local, data.Rand(16), string(compactNodes(nodes)))
	if err != nil {
		logging.Error("build get_peers not found response packet faield" + n.errInfo(err))
		return
	}
	_, err = n.dht.listen.WriteTo(data, &n.addr)
	if err != nil {
		logging.Error("send get_peers not found response packet failed" + n.errInfo(err))
		return
	}
}

func (n *node) onAnnouncePeer(buf []byte) {
	var req data.AnnouncePeerRequest
	err := bencode.Decode(buf, &req)
	if err != nil {
		logging.Error("decode announce_peer request failed" + n.errInfo(err))
		return
	}
	port := n.addr.Port
	if req.Data.Implied != 0 {
		port = int(req.Data.Port)
	}
	addr := net.TCPAddr{
		IP:   n.addr.IP,
		Port: port,
	}
	logging.Info("announce_peer found: hash=%x, addr=%s", req.Data.Hash, addr.String())
	data, err := data.AnnouncePeer(n.dht.local)
	if err != nil {
		logging.Error("build announce_peer response packet failed" + n.errInfo(err))
		return
	}
	_, err = n.dht.listen.WriteTo(data, &n.addr)
	if err != nil {
		logging.Error("send announce_peer packet failed" + n.errInfo(err))
		return
	}
}
