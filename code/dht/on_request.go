package dht

import (
	"bytes"
	"encoding/binary"

	"github.com/lwch/bencode"
	"github.com/lwch/magic/code/data"
	"github.com/lwch/magic/code/logging"
)

func (n *node) onPing(buf []byte) {
	var req data.Hdr
	err := bencode.Decode(buf, &req)
	if err != nil {
		logging.Error("decode ping request failed" + n.errInfo(err))
		return
	}
	data, err := data.PingRep(req.Transaction, n.dht.local)
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
	data, err := data.FindRep(req.Transaction, n.dht.local, string(compactNodes(nodes)))
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
	// raw, err := data.GetPeersNotFound(req.Transaction, req.Data.Hash, data.Rand(16), "")
	// if err != nil {
	// 	logging.Error("build get_peers not found response packet faield" + n.errInfo(err))
	// 	return
	// }
	// _, err = n.dht.listen.WriteTo(raw, &n.addr)
	// if err != nil {
	// 	logging.Error("send get_peers not found response packet failed" + n.errInfo(err))
	// 	return
	// }
	// return
	// logging.Info("get_peers: %x", req.Data.Hash)
	nodes := n.dht.tb.neighbor(req.Data.Hash)
	data, err := data.GetPeersNotFound(req.Transaction, n.dht.local, data.Rand(16), string(compactNodes(nodes)))
	if err != nil {
		logging.Error("build get_peers not found response packet faield" + n.errInfo(err))
		return
	}
	_, err = n.dht.listen.WriteTo(data, &n.addr)
	if err != nil {
		logging.Error("send get_peers not found response packet failed" + n.errInfo(err))
		return
	}
	if n.dht.even%2 == 1 {
		for _, node := range nodes {
			node.sendGet(req.Data.Hash)
		}
	}
	n.dht.even++
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
	data, err := data.AnnouncePeer(req.Transaction, n.dht.local)
	if err != nil {
		logging.Error("build announce_peer response packet failed" + n.errInfo(err))
		return
	}
	_, err = n.dht.listen.WriteTo(data, &n.addr)
	if err != nil {
		logging.Error("send announce_peer packet failed" + n.errInfo(err))
		return
	}
	n.dht.res.push(resReq{
		id:   req.Data.Hash,
		ip:   n.addr.IP,
		port: uint16(port),
	})
}
