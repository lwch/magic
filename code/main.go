package main

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"net"
	"time"
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
var ID string

func init() {
	rand.Seed(time.Now().UnixNano())
	ID = fmt.Sprintf("my name is magic%04d", rand.Intn(9999))
}

func ping(c *net.UDPConn) {
	data := "d1:ad2:id20:" + ID + "e1:q4:ping1:t2:aa1:y1:qe"
	_, err := c.Write([]byte(data))
	assert(err)
}

func find(c *net.UDPConn) {
	data := "d1:ad2:id20:" + ID + "6:target20:mnopqrstuvwxyz123456e1:q9:find_node1:t2:aa1:y1:qe"
	_, err := c.Write([]byte(data))
	assert(err)
}

// http://www.bittorrent.org/beps/bep_0005.html
func main() {
	remote, err := net.ResolveUDPAddr("udp", bootstrapAddrs[0])
	assert(err)
	c, err := net.DialUDP("udp", nil, remote)
	assert(err)
	defer c.Close()
	find(c)
	buf := make([]byte, 65535)
	for {
		n, err := c.Read(buf)
		assert(err)
		fmt.Println(hex.Dump(buf[:n]))
	}
}
