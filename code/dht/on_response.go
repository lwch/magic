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

func (mgr *NodeMgr) onDiscovery(node *Node, buf []byte) []*Node {
	var resp data.FindResponse
	err := bencode.Decode(buf, &resp)
	if err != nil {
		logging.Error("decode discovery data failed of %s, err=%v", node.HexID(), err)
		return nil
	}
	if len(resp.Response.Nodes)%26 > 0 {
		return nil
	}
	uniq := make(map[string]bool)
	ret := make([]*Node, 0, len(resp.Response.Nodes)/26)
	for i := 0; i < len(resp.Response.Nodes); i += 26 {
		if len(mgr.nodesAddr) >= mgr.maxNodes {
			// logging.Info("full nodes")
			return nil
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
		addrNode := mgr.nodesAddr[nextNode.AddrString()]
		if addrNode != nil {
			addrNode.Close()
		}
		if node := mgr.nodesID[nextNode.HexID()]; node != nil && node != addrNode {
			node.Close()
		}
		mgr.nodesAddr[nextNode.AddrString()] = nextNode
		mgr.nodesID[nextNode.HexID()] = nextNode
		mgr.Unlock()
		uniq[fmt.Sprintf("%x", next)] = true
		ret = append(ret, nextNode)
	}
	return ret
}

func (mgr *NodeMgr) onGetPeersResponse(node *Node, buf []byte, hash [20]byte) {
	var notfound data.GetPeersNotFoundResponse
	err := bencode.Decode(buf, &notfound)
	if err != nil {
		logging.Error("decode get_peers response data by not found failed of %s, err=%v", node.HexID(), err)
		return
	}
	if len(notfound.Response.Nodes) > 0 {
		nodes := mgr.onDiscovery(node, buf)
		for _, node := range nodes {
			if !mgr.rm.allowScan(hash) {
				break
			}
			node.sendGet(mgr.listen, mgr.id, hash)
			mgr.rm.scan(hash)
		}
		return
	}
	var found data.GetPeersResponse
	err = bencode.Decode(buf, &found)
	if err != nil {
		logging.Error("decode get_peers response data by found failed of %s, err=%v", node.HexID(), err)
		return
	}
	for _, peer := range found.Response.Values {
		var ip [4]byte
		var port uint16
		err = binary.Read(strings.NewReader(peer), binary.BigEndian, &ip)
		if err != nil {
			logging.Error("read ip failed of %s, err=%v", node.HexID(), err)
			continue
		}
		err = binary.Read(strings.NewReader(peer[4:]), binary.BigEndian, &port)
		if err != nil {
			logging.Error("read port failed of %s, err=%v", node.HexID(), err)
			continue
		}
		logging.Info("found: %x in %s:%d", hash, net.IP(ip[:]).String(), port)
	}
}
