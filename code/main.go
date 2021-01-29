package main

import (
	"math/rand"
	"net"
	"net/http"
	_ "net/http/pprof"
	"time"

	"github.com/lwch/magic/code/dht"
	"github.com/lwch/runtime"
)

var bootstrapAddrs []*net.UDPAddr

func init() {
	go func() {
		runtime.Assert(http.ListenAndServe(":6060", nil))
	}()
	rand.Seed(time.Now().UnixNano())
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
	cfg := dht.NewConfig()
	cfg.Listen = 6882
	cfg.MinNodes = 100000
	mgr, err := dht.New(cfg)
	runtime.Assert(err)
	mgr.Discovery(bootstrapAddrs)
	<-make(chan int)
}
