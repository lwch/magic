package dht

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/lwch/magic/code/data"
)

const neighborSize = 8

type hashType [20]byte

var emptyHash hashType

func (hash hashType) String() string {
	return fmt.Sprintf("%x", [20]byte(hash))
}

// DHT dht manager
type DHT struct {
	listen *net.UDPConn
	tb     *table
	tx     *txMgr
	local  hashType

	// runtime
	ctx    context.Context
	cancel context.CancelFunc
}

// New create dht manager
func New(cfg *Config) (*DHT, error) {
	cfg.checkDefault()
	dht := &DHT{
		tx:    newTXMgr(cfg.MaxTX),
		local: data.RandID(),
	}
	dht.tb = newTable(dht, cfg.MaxNodes)
	dht.ctx, dht.cancel = context.WithCancel(context.Background())
	var err error
	dht.listen, err = net.ListenUDP("udp", &net.UDPAddr{
		Port: int(cfg.Listen),
	})
	go dht.recv()
	return dht, err
}

// Close close object
func (dht *DHT) Close() {
	dht.listen.Close()
	dht.tb.close()
	dht.tx.close()
	dht.cancel()
}

// Discovery discovery nodes
func (dht *DHT) Discovery(addrs []*net.UDPAddr) {
	dht.bootstrap(addrs)
	tk := time.NewTicker(time.Second)
	for {
		select {
		case <-tk.C:
			dht.tb.onDiscovery(dht.listen)
		case <-dht.ctx.Done():
			return
		}
	}
}

func (dht *DHT) bootstrap(addrs []*net.UDPAddr) {
	for _, addr := range addrs {
		node := newNode(dht, data.RandID(), *addr)
		dht.tb.add(node)
	}
}

func (dht *DHT) recv() {
	buf := make([]byte, 65535)
	for {
		n, addr, err := dht.listen.ReadFrom(buf)
		if err != nil {
			continue
		}
		dht.handleData(addr, buf[:n])
	}
}

func (dht *DHT) handleData(addr net.Addr, buf []byte) {
	node := dht.tb.findAddr(addr)
	if node == nil {
		// TODO: log
		return
	}
	node.onRecv(buf)
}
