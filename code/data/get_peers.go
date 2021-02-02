package data

import "github.com/lwch/bencode"

// GetPeersRequest get_peers request
type GetPeersRequest struct {
	Hdr
	Action string `bencode:"q"`
	Data   struct {
		ID   [20]byte `bencode:"id"`
		Hash [20]byte `bencode:"info_hash"`
	} `bencode:"a"`
}

// GetPeersResponse get_peers response
type GetPeersResponse struct {
	Hdr
	Response struct {
		ID     [20]byte `bencode:"id"`
		Token  string   `bencode:"token"`
		Values []string `bencode:"values"`
	} `bencode:"r"`
}

// GetPeersNotFoundResponse get_peers response
type GetPeersNotFoundResponse struct {
	Hdr
	Response struct {
		ID    [20]byte `bencode:"id"`
		Token string   `bencode:"token"`
		Nodes string   `bencode:"nodes"`
	} `bencode:"r"`
}

// GetPeers build get_peers request packet
func GetPeers(id, hash [20]byte) ([]byte, string, error) {
	var req GetPeersRequest
	req.Hdr = newHdr(request)
	req.Action = "get_peers"
	req.Data.ID = id
	req.Data.Hash = hash
	data, err := bencode.Encode(req)
	if err != nil {
		return nil, "", err
	}
	return data, req.Hdr.Transaction, nil
}

// GetPeersNotFound build get_peers not found response packet
func GetPeersNotFound(tx string, id [20]byte, token, nodes string) ([]byte, error) {
	var rep GetPeersNotFoundResponse
	rep.Transaction = tx
	rep.Type = response
	rep.Response.ID = id
	rep.Response.Token = token
	rep.Response.Nodes = nodes
	data, err := bencode.Encode(rep)
	if err != nil {
		return nil, err
	}
	return data, nil
}
