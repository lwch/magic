package data

import "github.com/lwch/bencode"

type pingRequest struct {
	common
	query
	A struct {
		ID [20]byte `bencode:"id"`
	} `bencode:"a"`
}

// PingReq build ping request packet
func PingReq(target [20]byte) ([]byte, error) {
	p := pingRequest{
		common: newCommon(request),
		query: newQuery("ping", map[string][20]byte{
			"id": target,
		}),
	}
	return bencode.Encode(p)
}
