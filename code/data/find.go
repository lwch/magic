package data

import (
	"math/rand"

	"github.com/lwch/bencode"
)

// FindRequest find_node request
type FindRequest struct {
	Hdr
	query
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
func FindReq(target [20]byte) ([]byte, [20]byte, string, error) {
	var id [20]byte
	rand.Read(id[:])
	hdr := newHdr(request)
	data, err := bencode.Encode(FindRequest{
		Hdr: hdr,
		query: newQuery("find_node", map[string][20]byte{
			"id":     id,
			"target": target,
		}),
	})
	if err != nil {
		return nil, id, "", err
	}
	return data, id, hdr.Transaction, nil
}
