package dht

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"time"

	"github.com/lwch/bencode"
	"github.com/lwch/magic/code/data"
	"github.com/lwch/magic/code/logging"
)

// Find find_node
func Find(mgr *NodeMgr, id [20]byte, addr *net.UDPAddr) ([]*Node, error) {
	c, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil, err
	}
	defer c.Close()
	var next [20]byte
	rand.Read(next[:])
	find, localID, _, err := data.FindReq(next)
	if err != nil {
		return nil, err
	}
	_, err = c.Write(find)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, 65535)
	c.SetReadDeadline(time.Now().Add(time.Second))
	n, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	var findResp data.FindResponse
	err = bencode.Decode(buf[:n], &findResp)
	if err != nil {
		return nil, err
	}
	list := make([]*Node, 0, len(findResp.Response.Nodes)/26)
	uniq := make(map[string]bool)
	for i := 0; i < len(findResp.Response.Nodes); i += 26 {
		var ip [4]byte
		var port uint16
		err = binary.Read(strings.NewReader(findResp.Response.Nodes[i+20:]), binary.BigEndian, &ip)
		if err != nil {
			continue
		}
		err = binary.Read(strings.NewReader(findResp.Response.Nodes[i+24:]), binary.BigEndian, &port)
		if err != nil {
			continue
		}
		copy(next[:], findResp.Response.Nodes[i:i+20])
		if uniq[fmt.Sprintf("%x", next)] {
			continue
		}
		node, err := newNode(mgr, localID, next, net.UDPAddr{
			IP:   net.IP(ip[:]),
			Port: int(port),
		})
		if err != nil {
			continue
		}
		logging.Info("find node %s, addr=%s", node.HexID(), node.C().RemoteAddr())
		list = append(list, node)
		uniq[fmt.Sprintf("%x", next)] = true
	}
	return list, nil
}
