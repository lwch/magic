package main

import (
	"fmt"
	"math/rand"
	"net"
	"net/http"
	_ "net/http/pprof"
	"time"

	"github.com/lwch/magic/code/dht"
	"github.com/lwch/runtime"
)

var bootstrapAddrs []*net.UDPAddr

// ID random id
var ID [20]byte

func init() {
	go func() {
		runtime.Assert(http.ListenAndServe(":6060", nil))
	}()
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

func main() {
	mgr := dht.NewNodeMgr(ID, 3000)
	mgr.Discovery(bootstrapAddrs)
}
