package dht

import (
	"bytes"
	"encoding/binary"
	"net"
	"strings"
	"time"

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
		var id hashType
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
		addr := net.UDPAddr{
			IP:   net.IP(ip[:]),
			Port: int(port),
		}
		go func(node *node) {
			tx := node.sendPing()
			n.dht.init.push(tx, node)
			defer n.dht.init.unset(tx)
			select {
			case <-node.chPong:
				n.dht.tb.add(node)
			case <-time.After(10 * time.Second):
				return
			}
		}(newNode(n.dht, id, addr))
	}
}

func (n *node) onGetPeersResp(buf []byte, hash hashType) {
	if bytes.Equal(hash[:], emptyHash[:]) {
		return
	}
	var notfound data.GetPeersNotFoundResponse
	err := bencode.Decode(buf, &notfound)
	if err != nil {
		logging.Error("decode get_peers response(notfound) failed" + n.errInfo(err))
		return
	}
	if len(notfound.Response.Nodes) > 0 {
		n.onFindNodeResp(buf)
		return
	}
	// var found data.GetPeersResponse
	// err = bencode.Decode(buf, &found)
	// if err != nil {
	// 	logging.Error("decode get_peers response(found) failed" + n.errInfo(err))
	// 	return
	// }
	// // n.dht.tk.add(found.Response.Token, hash, n.id)
	// for _, peer := range found.Response.Values {
	// 	if len(peer) != 6 {
	// 		continue
	// 	}
	// 	var ip [4]byte
	// 	err = binary.Read(strings.NewReader(peer), binary.BigEndian, &ip)
	// 	if err != nil {
	// 		logging.Error("read ip failed" + n.errInfo(err))
	// 		continue
	// 	}
	// 	port := binary.BigEndian.Uint16([]byte(peer[4:]))
	// 	n.dht.res.push(resReq{
	// 		id:   hash,
	// 		ip:   net.IP(ip[:]),
	// 		port: port,
	// 	})
	// }
}
