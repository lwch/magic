package dht

import (
	"encoding/binary"
	"fmt"
	"net"
	"strings"

	"github.com/lwch/bencode"
	"github.com/lwch/magic/code/data"
	"github.com/lwch/magic/code/logging"
)

func (mgr *NodeMgr) onDiscovery(node *Node, buf []byte) {
	var resp data.FindResponse
	err := bencode.Decode(buf, &resp)
	if err != nil {
		logging.Error("decode discovery data failed of %s, err=%v", node.HexID(), err)
		return
	}
	uniq := make(map[string]bool)
	for i := 0; i < len(resp.Response.Nodes); i += 26 {
		if len(mgr.nodes) >= mgr.maxNodes {
			logging.Info("full nodes")
			return
		}
		var ip [4]byte
		var port uint16
		err = binary.Read(strings.NewReader(resp.Response.Nodes[i+20:]), binary.BigEndian, &ip)
		if err != nil {
			logging.Error("read ip failed of %s, err=%v", node.HexID(), err)
			continue
		}
		err = binary.Read(strings.NewReader(resp.Response.Nodes[i+24:]), binary.BigEndian, &port)
		if err != nil {
			logging.Error("read port failed of %s, err=%v", node.HexID(), err)
			continue
		}
		if port == 0 {
			continue
		}
		var next [20]byte
		copy(next[:], resp.Response.Nodes[i:i+20])
		if uniq[fmt.Sprintf("%x", next)] {
			continue
		}
		if mgr.Exists(next) {
			continue
		}
		addr := net.UDPAddr{
			IP:   net.IP(ip[:]),
			Port: int(port),
		}
		nextNode := newNode(mgr, mgr.id, next, addr)
		logging.Debug("discovery node %s, addr=%s", node.HexID(), node.AddrString())
		mgr.Lock()
		if node := mgr.nodes[nextNode.AddrString()]; node != nil {
			node.Close()
		}
		mgr.nodes[nextNode.AddrString()] = nextNode
		mgr.Unlock()
		uniq[fmt.Sprintf("%x", next)] = true
	}
}

func (mgr *NodeMgr) onGetPeersResponse(node *Node, buf []byte) {
	var notfound data.GetPeersNotFoundResponse
	err := bencode.Decode(buf, &notfound)
	if err != nil {
		logging.Error("decode get_peers response data by not found failed of %s, err=%v", node.HexID(), err)
		return
	}
	logging.Info("notfound=%s", notfound.Response.Nodes)
}
