package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"math/rand"
	"net"
	"time"

	"github.com/lwch/magic/code/dht"
	"github.com/lwch/magic/code/logging"
	"github.com/lwch/runtime"
	_ "github.com/mattn/go-sqlite3"
)

var bootstrapAddrs []*net.UDPAddr

func init() {
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
	listen := flag.Uint("listen", 6881, "listen port")
	minNodes := flag.Int("min-nodes", 100000, "minimum nodes in descovery")
	maxNodes := flag.Int("max-nodes", 1000000, "maximum nodes in descovery")
	dbAddr := flag.String("db", "data.db", "sqlite save dir")
	flag.Parse()

	db, err := sql.Open("sqlite3", "file:"+*dbAddr+"?cache=shared")
	runtime.Assert(err)
	defer db.Close()
	dbInit(db)

	run(uint16(*listen), *minNodes, *maxNodes, db)
}

func dbInit(db *sql.DB) {
	exec := func(qry string, args ...interface{}) {
		_, err := db.Exec(qry, args...)
		runtime.Assert(err)
	}
	exec(`CREATE TABLE IF NOT EXISTS resource(
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		created integer,
		hash text NOT NULL,
		name text NOT NULL,
		length integer,
		data text NOT NULL
	)`)
	exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_hash ON resource(hash)`)
}

func run(listen uint16, minNodes, maxNodes int, db *sql.DB) {
	cfg := dht.NewConfig()
	cfg.Listen = listen
	cfg.MinNodes = minNodes
	cfg.MaxNodes = maxNodes
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
	for info := range mgr.Out {
		data, _ := json.Marshal(info)
		logging.Info("info: %s", string(data))
		_, err = db.Exec("INSERT OR IGNORE INTO resource(created, hash, name, length, data) VALUES(?, ?, ?, ?, ?)",
			time.Now().Unix(), info.Hash, info.Name, info.Length, string(data))
		if err != nil {
			logging.Error("log resource failed, hash=%s, err=%v", info.Hash, err)
		}
	}
}
