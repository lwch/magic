package dht

import (
	"bytes"
	"context"
	"crypto/rand"
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
	init   *initQueue
	// bl     *blacklist
	local hashType

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
		// bl: newBlackList(),
		init: newInitQueue(),
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
	for _, addr := range addrs {
		node := newBootstrapNode(dht, *addr)
		node.sendDiscovery(dht.listen, dht.local)
		dht.tb.add(node)
	}
	dht.tb.discovery()
}

func (dht *DHT) recv() {
	buf := make([]byte, 65535)
	for {
		n, addr, err := dht.listen.ReadFrom(buf)
		if err != nil {
			continue
		}
		// if dht.bl.isBlockAddr(addr) {
		// 	continue
		// }
		dht.handleData(addr, buf[:n])
	}
}

func (dht *DHT) handleData(addr net.Addr, buf []byte) {
	var hdr data.Hdr
	err := bencode.Decode(buf, &hdr)
	if err != nil {
		return
	}
	node := dht.tb.findAddr(addr)
	if node == nil {
		var req struct {
			Data struct {
				ID [20]byte `bencode:"id"`
			} `bencode:"a"`
		}
		switch {
		case hdr.IsRequest():
			// if dht.bl.isBlockID(req.Data.ID) {
			// 	return
			// }
			node = newNode(dht, req.Data.ID, *addr.(*net.UDPAddr))
			logging.Debug("anonymous node: %x, addr=%s", req.Data.ID, addr.String())
			dht.tb.add(node)
			// } else if dht.bl.isBlockID(node.id) {
			// 	return
		case hdr.IsResponse():
			node := dht.init.find(hdr.Transaction)
			if node == nil {
				// TODO: block
				return
			}
			node.updated = time.Now()
			node.pong = time.Now()
			dht.tb.add(node)
			dht.init.unset(hdr.Transaction)
			return
		default:
			return
		}
	}
	node.onRecv(buf)
}
