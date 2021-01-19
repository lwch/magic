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

// GetPeersNotFoundResponse find response
type GetPeersNotFoundResponse struct {
	Hdr
	Response struct {
		ID    [20]byte `bencode:"id"`
		Token string   `bencode:"token"`
		Nodes string   `bencode:"nodes"`
	} `bencode:"r"`
}

// GetPeersNotFound build get_peers not found response packet
func GetPeersNotFound(id [20]byte, token, nodes string) ([]byte, error) {
	var rep GetPeersNotFoundResponse
	rep.Hdr = newHdr(response)
	rep.Response.ID = id
	rep.Response.Token = token
	rep.Response.Nodes = nodes
	data, err := bencode.Encode(rep)
	if err != nil {
		return nil, err
	}
	return data, nil
}
