package data

import (
	"crypto/rand"

	"github.com/lwch/bencode"
)

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
func PingReq(id [20]byte) ([]byte, string, error) {
	// optimize
	var tx [16]byte
	rand.Read(tx[:])
	data := []byte("d1:y1:q1:q4:ping1:t16:")
	data = append(data, tx[:]...)
	data = append(data, []byte("1:ad2:id20:")...)
	data = append(data, id[:]...)
	data = append(data, []byte("ee")...)
	return data, string(tx[:]), nil
	// req := PingRequest{
	// 	Hdr: newHdr(request),
	// 	reqData: newReqData("ping", map[string][20]byte{
	// 		"id": id,
	// 	}),
	// }
	// data, err := bencode.Encode(req)
	// if err != nil {
	// 	return nil, "", err
	// }
	// return data, req.Hdr.Transaction, nil
}

// PingRep build ping response packet
func PingRep(id [20]byte) ([]byte, error) {
	return bencode.Encode(PingResponse{
		Hdr: newHdr(response),
	})
}
