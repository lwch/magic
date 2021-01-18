package data

import "github.com/lwch/bencode"

// PingRequest ping request
type PingRequest struct {
	Hdr
	query
}

// PingResponse ping response
type PingResponse struct {
	Hdr
	Response struct {
		ID [20]byte `bencode:"id"`
	} `bencode:"r"`
}

// PingReq build ping request packet
func PingReq(id [20]byte) ([]byte, error) {
	return bencode.Encode(PingRequest{
		Hdr: newHdr(request),
		query: newQuery("ping", map[string][20]byte{
			"id": id,
		}),
	})
}
