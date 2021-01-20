package data

import "github.com/lwch/bencode"

const (
	request  = "q"
	response = "r"
	err      = "e"
)

// ReqType request type
type ReqType string

const (
	// TypePing ping
	TypePing ReqType = "ping"
	// TypeFindNode find_node
	TypeFindNode ReqType = "find_node"
	// TypeGetPeers get_peers
	TypeGetPeers ReqType = "get_peers"
	// TypeAnnouncePeer announce_peer
	TypeAnnouncePeer ReqType = "announce_peer"
)

// Hdr bencode header
type Hdr struct {
	Transaction string `bencode:"t"`
	Type        string `bencode:"y"`
}

// IsRequest is request packet
func (h Hdr) IsRequest() bool {
	return h.Type == request
}

// IsResponse is response packet
func (h Hdr) IsResponse() bool {
	return h.Type == response
}

type reqData struct {
	Action string      `bencode:"q"`
	Data   interface{} `bencode:"a"`
}

type repData struct {
	Data interface{} `bencode:"r"`
}

func newHdr(t string) Hdr {
	return Hdr{
		Transaction: Rand(16),
		Type:        t,
	}
}

func newReqData(action string, data interface{}) reqData {
	return reqData{
		Action: action,
		Data:   data,
	}
}

func newRepData(data interface{}) repData {
	return repData{Data: data}
}

// ParseReqType parse request type
func ParseReqType(data []byte) ReqType {
	var t struct {
		Type ReqType `bencode:"q"`
	}
	bencode.Decode(data, &t)
	return t.Type
}
