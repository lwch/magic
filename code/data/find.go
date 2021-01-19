package data

import (
	"github.com/lwch/bencode"
)

// FindRequest find_node request
type FindRequest struct {
	Hdr
	Action string `bencode:"q"`
	Data   struct {
		ID     [20]byte `bencode:"id"`
		Target [20]byte `bencode:"target"`
	} `bencode:"a"`
}

// FindResponse find response
type FindResponse struct {
	Hdr
	Response struct {
		ID    [20]byte `bencode:"id"`
		Nodes string   `bencode:"nodes"`
	} `bencode:"r"`
}

// FindReq build find_node request packet
func FindReq(id, target [20]byte) ([]byte, string, error) {
	var req FindRequest
	req.Hdr = newHdr(request)
	req.Action = "find_node"
	req.Data.ID = id
	req.Data.Target = target
	data, err := bencode.Encode(req)
	if err != nil {
		return nil, "", err
	}
	return data, req.Hdr.Transaction, nil
}

// FindRep build find_node response packet
func FindRep(id [20]byte, nodes string) ([]byte, error) {
	var rep FindResponse
	rep.Hdr = newHdr(response)
	rep.Response.ID = id
	rep.Response.Nodes = nodes
	data, err := bencode.Encode(rep)
	if err != nil {
		return nil, err
	}
	return data, nil
}
