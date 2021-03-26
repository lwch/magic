package main

import (
	"encoding/json"
	"math/rand"
	"net"
	"net/http"
	_ "net/http/pprof"
	"time"

	"github.com/lwch/magic/code/dht"
	"github.com/lwch/magic/code/logging"
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
	cfg.MinNodes = 100000
	cfg.NodeFilter = func(ip net.IP, id [20]byte) bool {
		return false
	}
	mgr, err := dht.New(cfg)
	runtime.Assert(err)
	mgr.Discovery(bootstrapAddrs)
	var nodes int
	go func() {
		for count := range mgr.Nodes {
			nodes = count
		}
	}()
	go func() {
		for {
			time.Sleep(10 * time.Second)
			logging.Info("%d nodes", nodes)
		}
	}()
	uniq := make(map[string]bool)
	for info := range mgr.Out {
		if uniq[info.Hash] {
			continue
		}
		data, _ := json.Marshal(info)
		logging.Info("info: %s", string(data))
		uniq[info.Hash] = true
	}
}
