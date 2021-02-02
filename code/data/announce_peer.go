package data

import "github.com/lwch/bencode"

// AnnouncePeerRequest announce_peer request
type AnnouncePeerRequest struct {
	Hdr
	Action string `bencode:"q"`
	Data   struct {
		ID      [20]byte `bencode:"id"`
		Hash    [20]byte `bencode:"info_hash"`
		Implied int      `bencode:"implied_port"`
		Port    uint16   `bencode:"port"`
	} `bencode:"a"`
}

// AnnouncePeerResponse announce_peer response
type AnnouncePeerResponse struct {
	Hdr
	Response struct {
		ID [20]byte `bencode:"id"`
	} `bencode:"r"`
}

// AnnouncePeer build announce_peer response packet
func AnnouncePeer(tx string, id [20]byte) ([]byte, error) {
	var rep AnnouncePeerResponse
	rep.Transaction = tx
	rep.Type = response
	rep.Response.ID = id
	data, err := bencode.Encode(rep)
	if err != nil {
		return nil, err
	}
	return data, nil
}
