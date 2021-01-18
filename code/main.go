package main

import (
	"fmt"
	"math/rand"
	"net"
	"time"

	"github.com/lwch/magic/code/dht"
	"github.com/lwch/runtime"
)

var bootstrapAddrs []*net.UDPAddr

// ID random id
var ID [20]byte

func init() {
	rand.Seed(time.Now().UnixNano())
	copy(ID[:], fmt.Sprintf("my name is magic%04d", rand.Intn(9999)))
	for _, addr := range []string{
		"router.bittorrent.com:6881",
		"router.utorrent.com:6881",
		"dht.transmissionbt.com:6881",
	} {
		addr, err := net.ResolveUDPAddr("udp", addr)
		runtime.Assert(err)
		bootstrapAddrs = append(bootstrapAddrs, addr)
	}
}

// http://www.bittorrent.org/beps/bep_0005.html
func main() {
	mgr := dht.NewNodeMgr(ID, 3000)
	for {
		for _, addr := range bootstrapAddrs {
			nodes, err := dht.Find(mgr, ID, addr)
			if err != nil {
				continue
			}
			for _, node := range nodes {
				if !mgr.Push(node) {
					node.Close()
					continue
				}
			}
		}
		time.Sleep(time.Second)
	}
}
