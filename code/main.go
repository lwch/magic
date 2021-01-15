package main

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"net"
	"time"

	"github.com/lwch/bencode"
	"github.com/lwch/magic/code/data"
)

var bootstrapAddrs = []string{
	"router.bittorrent.com:6881",
	"router.utorrent.com:6881",
	"dht.transmissionbt.com:6881",
}

func assert(err error) {
	if err != nil {
		panic(err)
	}
}

// ID random id
var ID [20]byte

func init() {
	rand.Seed(time.Now().UnixNano())
	copy(ID[:], fmt.Sprintf("my name is magic%04d", rand.Intn(9999)))
}

// http://www.bittorrent.org/beps/bep_0005.html
func main() {
	remote, err := net.ResolveUDPAddr("udp", bootstrapAddrs[0])
	assert(err)
	c, err := net.DialUDP("udp", nil, remote)
	assert(err)
	defer c.Close()
	ping, err := data.PingReq(ID)
	assert(err)
	_, err = c.Write(ping)
	assert(err)
	buf := make([]byte, 65535)
	n, err := c.Read(buf)
	assert(err)
	fmt.Println(hex.Dump(buf[:n]))
	var pingResp data.PingResponse
	assert(bencode.Decode(buf[:n], &pingResp))
	fmt.Println(pingResp)
	for {
		n, err := c.Read(buf)
		assert(err)
		fmt.Println(hex.Dump(buf[:n]))
	}
}
