package data

import "github.com/lwch/bencode"

// PingRequest ping request
type PingRequest struct {
	common
	query
}

// PingResponse ping response
type PingResponse struct {
	common
	Response struct {
		ID [20]byte `bencode:"id"`
	} `bencode:"a"`
}

// PingReq build ping request packet
func PingReq(target [20]byte) ([]byte, error) {
	return bencode.Encode(PingRequest{
		common: newCommon(request),
		query: newQuery("ping", map[string][20]byte{
			"id": target,
		}),
	})
}
