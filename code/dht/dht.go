package dht

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"time"

	"github.com/lwch/bencode"
	"github.com/lwch/magic/code/data"
	"github.com/lwch/magic/code/logging"
)

const neighborSize = 8

type hashType [20]byte

var emptyHash hashType

func (hash hashType) String() string {
	return fmt.Sprintf("%x", [20]byte(hash))
}

func (hash hashType) raw() [20]byte {
	return [20]byte(hash)
}

func (hash hashType) equal(h hashType) bool {
	a := hash.raw()
	b := h.raw()
	return bytes.Equal(a[:], b[:])
}

// DHT dht manager
type DHT struct {
	listen *net.UDPConn
	tb     *table
	tx     *txMgr
	tk     *tokenMgr
	bc     *broadcast
	local  hashType

	// runtime
	ctx    context.Context
	cancel context.CancelFunc
}

// New create dht manager
func New(cfg *Config) (*DHT, error) {
	cfg.checkDefault()
	dht := &DHT{
		tx: newTXMgr(cfg.TxTimeout),
		tk: newTokenMgr(cfg.MaxToken),
		bc: newBroadcast(cfg.MaxBroadcastCache, cfg.MaxBroadcastCount),
	}
	rand.Read(dht.local[:])
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
	dht.tk.close()
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
		if bytes.Contains(buf[:n], []byte("announce_peer")) {
			logging.Info("addr=%s\n%s", addr.String(), hex.Dump(buf[:n]))
		}
		dht.handleData(addr, buf[:n])
	}
}

func (dht *DHT) handleData(addr net.Addr, buf []byte) {
	node := dht.tb.findAddr(addr)
	if node == nil {
		var req struct {
			data.Hdr
			Data struct {
				ID [20]byte `bencode:"id"`
			} `bencode:"a"`
		}
		err := bencode.Decode(buf, &req)
		if err != nil {
			return
		}
		if !req.Hdr.IsRequest() {
			return
		}
		node = newNode(dht, req.Data.ID, *addr.(*net.UDPAddr))
		dht.tb.addForce(node)
		logging.Debug("anonymous node: %x, addr=%s", req.Data.ID, addr.String())
	}
	node.onRecv(buf)
}
