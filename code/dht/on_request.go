package dht

import (
	"bytes"
	"encoding/binary"

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
	nodes := n.dht.tb.neighbor(req.Data.Target, neighborSize)
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
