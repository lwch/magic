package dht

import (
	"encoding/binary"
	"net"
	"strings"

	"github.com/lwch/bencode"
	"github.com/lwch/magic/code/data"
	"github.com/lwch/magic/code/logging"
)

func (n *node) onFindNodeResp(buf []byte) {
	var resp data.FindResponse
	err := bencode.Decode(buf, &resp)
	if err != nil {
		logging.Error("decode find_node response data failed, id=%s, addr=%s, err=%v",
			n.id.String(), n.addr.String(), err)
		return
	}
	if len(resp.Response.Nodes)%26 > 0 {
		logging.Error("invalid find_node response node data length, id=%s, addr=%s",
			n.id.String(), n.addr.String())
		return
	}
	for i := 0; i < len(resp.Response.Nodes); i += 26 {
		if n.dht.tb.isFull() {
			return
		}
		var id idType
		copy(id[:], resp.Response.Nodes[i:i+20])
		if n.dht.tb.findID(id) != nil {
			continue
		}
		var ip [4]byte
		err = binary.Read(strings.NewReader(resp.Response.Nodes[i+20:]), binary.BigEndian, &ip)
		if err != nil {
			logging.Error("read ip failed, id=%s, addr=%s, err=%v",
				n.id.String(), n.addr.String(), err)
			continue
		}
		port := binary.BigEndian.Uint16([]byte(resp.Response.Nodes[i+24:]))
		if port == 0 {
			continue
		}
		node := newNode(n.dht, id, net.UDPAddr{
			IP:   net.IP(ip[:]),
			Port: int(port),
		})
		n.dht.tb.add(node)
	}
}
