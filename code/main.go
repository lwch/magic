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
	"github.com/oschwald/geoip2-golang"
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
	geo, err := geoip2.Open("GeoLite2-Country.mmdb")
	runtime.Assert(err)
	cfg := dht.NewConfig()
	cfg.MinNodes = 100000
	var names = []string{
		"AG", // Ares
		"A~", // Ares
		"AR", // Arctic
		"AV", // Avicora
		"AX", // BitPump
		"AZ", // Azureus
		"BB", // BitBuddy
		"BC", // BitComet
		"BF", // Bitflu
		"BG", // BTG (uses Rasterbar libtorrent)
		"BR", // BitRocket
		"BS", // BTSlave
		"BX", // ~Bittorrent X
		"CD", // Enhanced CTorrent
		"CT", // CTorrent
		"DE", // DelugeTorrent
		"DP", // Propagate Data Client
		"EB", // EBit
		"ES", // electric sheep
		"FT", // FoxTorrent
		"FW", // FrostWire
		"FX", // Freebox BitTorrent
		"GS", // GSTorrent
		"HL", // Halite
		"HN", // Hydranode
		"KG", // KGet
		"KT", // KTorrent
		"LH", // LH-ABC
		"LP", // Lphant
		"LT", // libtorrent
		"lt", // libTorrent
		"LW", // LimeWire
		"MO", // MonoTorrent
		"MP", // MooPolice
		"MR", // Miro
		"MT", // MoonlightTorrent
		"NX", // Net Transport
		"PD", // Pando
		"qB", // qBittorrent
		"QD", // QQDownload
		"QT", // Qt 4 Torrent example
		"RT", // Retriever
		"S~", // Shareaza alpha/beta
		"SB", // ~Swiftbit
		"SS", // SwarmScope
		"ST", // SymTorrent
		"st", // sharktorrent
		"SZ", // Shareaza
		"TN", // TorrentDotNET
		"TR", // Transmission
		"TS", // Torrentstorm
		"TT", // TuoTu
		"UL", // uLeecher!
		"UT", // µTorrent
		"UW", // µTorrent Web
		"VG", // Vagaa
		"WD", // WebTorrent Desktop
		"WT", // BitLet
		"WW", // WebTorrent
		"WY", // FireTorrent
		"XL", // Xunlei
		"XT", // XanTorrent
		"XX", // Xtorrent
		"ZT", // ZipTorrent
	}
	idxName := 0
	maxNames := len(names)
	cfg.GenID = func() [20]byte {
		n := idxName % maxNames
		idxName++

		var id [20]byte
		id[0] = '-'
		id[1] = names[n][0]
		id[2] = names[n][1]
		rand.Read(id[3:])
		for i := 3; i < 7; i++ {
			id[i] = '0' + id[i]%10
		}
		id[7] = '-'
		return id
	}
	cfg.NodeFilter = func(ip net.IP, id [20]byte) bool {
		country, err := geo.Country(ip)
		if err != nil {
			return true
		}
		return country.Continent.Names["zh-CN"] == "亚洲"
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
