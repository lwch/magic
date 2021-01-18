package data

import "github.com/lwch/bencode"

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
func FindReq(id, target [20]byte) ([]byte, string, error) {
	hdr := newHdr(request)
	data, err := bencode.Encode(FindRequest{
		Hdr: hdr,
		query: newQuery("find_node", map[string][20]byte{
			"id":     id,
			"target": target,
		}),
	})
	if err != nil {
		return nil, "", err
	}
	return data, hdr.Transaction, nil
}
