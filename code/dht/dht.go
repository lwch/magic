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

func (hash hashType) bit(n int) byte {
	if n < 0 || n >= len(hash)*8 {
		panic(fmt.Errorf("out of range 0~%d[%d]", len(hash)*8-1, n))
	}
	bt := n / 8
	bit := n % 8
	if bit > 0 {
		return (hash[bt] >> (7 - bit)) & 1
	}
	return hash[bt] >> 7
}

type pkt struct {
	data []byte
	addr net.Addr
}

// DHT dht manager
type DHT struct {
	listen   *net.UDPConn
	tb       *table
	tx       *txMgr
	init     *initQueue
	local    hashType
	chRead   chan pkt
	maxNodes int

	// runtime
	ctx    context.Context
	cancel context.CancelFunc
}

// New create dht manager
func New(cfg *Config) (*DHT, error) {
	cfg.checkDefault()
	dht := &DHT{
		tx:       newTXMgr(cfg.TxTimeout),
		init:     newInitQueue(cfg.MaxNodes),
		chRead:   make(chan pkt, 100),
		maxNodes: cfg.MaxNodes,
	}
	rand.Read(dht.local[:])
	dht.tb = newTable(dht, cfg.MaxNodes)
	dht.ctx, dht.cancel = context.WithCancel(context.Background())
	var err error
	dht.listen, err = net.ListenUDP("udp", &net.UDPAddr{
		Port: int(cfg.Listen),
	})
	go dht.recv()
	go dht.handler()
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
		data := make([]byte, n)
		copy(data, buf[:n])
		select {
		case dht.chRead <- pkt{
			data: data,
			addr: addr,
		}:
		default:
		}
	}
}

func (dht *DHT) handler() {
	tk := time.Tick(time.Second)
	for {
		select {
		case pkt := <-dht.chRead:
			dht.handleData(pkt.addr, pkt.data)
		case <-tk:
			if dht.tb.size == 0 {
				dht.tb.discovery()
			} else if dht.tx.size() == 0 {
				dht.tb.discovery()
			}
		case <-dht.ctx.Done():
			return
		}
	}
}

func (dht *DHT) handleData(addr net.Addr, buf []byte) {
	node := dht.tb.findAddr(addr)
	if node == nil {
		var hdr data.Hdr
		err := bencode.Decode(buf, &hdr)
		if err != nil {
			return
		}
		switch {
		case hdr.IsRequest():
			return
		case hdr.IsResponse():
			node = dht.init.find(hdr.Transaction)
			if node == nil {
				return
			}
			node.updated = time.Now()
			select {
			case node.chPong <- struct{}{}:
			default:
			}
			return
		default:
			return
		}
	}
	node.onRecv(buf)
}
