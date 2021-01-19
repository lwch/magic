package data

import "github.com/lwch/bencode"

// PingRequest ping request
type PingRequest struct {
	Hdr
	reqData
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
		reqData: newReqData("ping", map[string][20]byte{
			"id": id,
		}),
	})
}

// PingRep build ping response packet
func PingRep(id [20]byte) ([]byte, error) {
	return bencode.Encode(PingResponse{
		Hdr: newHdr(response),
	})
}
