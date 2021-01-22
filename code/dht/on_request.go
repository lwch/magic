package dht

import (
	"bytes"

	"github.com/lwch/bencode"
	"github.com/lwch/magic/code/data"
	"github.com/lwch/magic/code/logging"
)

func (mgr *NodeMgr) onPing(node *Node, buf []byte) {
	data, err := data.PingRep(mgr.id)
	if err != nil {
		logging.Error("build ping response packet failed of %s, err=%v", node.HexID(), err)
		return
	}
	_, err = mgr.listen.WriteTo(data, &node.addr)
	if err != nil {
		logging.Error("send ping response packet failed of %s, err=%v", node.HexID(), err)
		return
	}
}

func (mgr *NodeMgr) onFindNode(node *Node, buf []byte) {
	var req data.FindRequest
	err := bencode.Decode(buf, &req)
	if err != nil {
		logging.Error("parse find_node packet failed of %s, err=%v", node.HexID(), err)
		return
	}
	nodes := mgr.topK(req.Data.Target, topSize)
	if nodes == nil {
		logging.Info("less nodes")
		return
	}
	data, err := data.FindRep(mgr.id, string(formatNodes(nodes)))
	if err != nil {
		logging.Error("build find_node response packet failed of %s, err=%v", node.HexID(), err)
		return
	}
	_, err = mgr.listen.WriteTo(data, &node.addr)
	if err != nil {
		logging.Error("send find_node response packet failed of %s, err=%v", node.HexID(), err)
		return
	}
}

var emptyHash [20]byte

func (mgr *NodeMgr) onGetPeers(node *Node, buf []byte) {
	var req data.GetPeersRequest
	err := bencode.Decode(buf, &req)
	if err != nil {
		logging.Error("parse get_peers packet failed of %s, err=%v", node.HexID(), err)
		return
	}
	logging.Info("get_peers: %x", req.Data.Hash)
	nodes := mgr.topK(req.Data.Hash, topSize)
	if nodes == nil {
		logging.Info("less nodes")
		return
	}
	data, err := data.GetPeersNotFound(mgr.id, data.Rand(32), string(formatNodes(nodes)))
	if err != nil {
		logging.Error("build get_peers response packet failed of %s, err=%v", node.HexID(), err)
		return
	}
	_, err = mgr.listen.WriteTo(data, &node.addr)
	if err != nil {
		logging.Error("send get_peers response packet failed of %s, err=%v", node.HexID(), err)
		return
	}
	if bytes.Equal(req.Data.Hash[:], emptyHash[:]) {
		return
	}
	for _, node := range nodes {
		if !mgr.rm.allowScan(req.Data.Hash) {
			break
		}
		node.sendGet(mgr.listen, mgr.id, req.Data.Hash)
		mgr.rm.scan(req.Data.Hash)
	}
}

func (mgr *NodeMgr) onAnnouncePeer(node *Node, buf []byte) {
	var req data.AnnouncePeerRequest
	err := bencode.Decode(buf, &req)
	if err != nil {
		logging.Error("parse announce_peer packet failed of %s, err=%v", node.HexID(), err)
		return
	}
	logging.Info("announce_peer: %x", req.Data.Hash)
	data, err := data.AnnouncePeer(mgr.id)
	if err != nil {
		logging.Error("build announce_peer response packet failed of %s, err=%v", node.HexID(), err)
		return
	}
	_, err = mgr.listen.WriteTo(data, &node.addr)
	if err != nil {
		logging.Error("send announce_peer response packet failed of %s, err=%v", node.HexID(), err)
		return
	}
	mgr.rm.markFound(req.Data.Hash, node.addr.IP, req.Data.Port)
}
