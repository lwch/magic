package dht

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/lwch/bencode"
	"github.com/lwch/magic/code/data"
	"github.com/lwch/magic/code/logging"
)

const neighborSize = 8
const maxDiscoverySize = 32

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
	res      *resMgr
	local    hashType
	chRead   chan pkt
	minNodes int
	even     int           // speed control
	Out      chan MetaInfo // discovery file info
	Nodes    chan int      // current node count
	nodePool sync.Pool
	gen      func() [20]byte

	// runtime
	ctx    context.Context
	cancel context.CancelFunc
}

// New create dht manager
func New(cfg *Config) (*DHT, error) {
	cfg.checkDefault()
	dht := &DHT{
		local:    data.RandID(),
		tx:       newTXMgr(cfg.TxTimeout),
		init:     newInitQueue(),
		chRead:   make(chan pkt, 1000),
		minNodes: cfg.MinNodes,
		Out:      make(chan MetaInfo),
		Nodes:    make(chan int),
		gen:      cfg.GenID,
	}
	dht.nodePool = sync.Pool{
		New: func() interface{} {
			return &node{dht: dht}
		},
	}
	// rand.Read(dht.local[:])
	dht.tb = newTable(dht, neighborSize, cfg.MaxNodes, cfg.GenID, cfg.NodeFilter)
	dht.res = newResMgr(dht)
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
	dht.res.close()
	dht.cancel()
}

// Discovery discovery nodes
func (dht *DHT) Discovery(addrs []*net.UDPAddr) {
	for _, addr := range addrs {
		node := newBootstrapNode(dht, *addr)
		node.sendDiscovery(dht.gen)
		dht.tb.add(node)
	}
	dht.tb.discovery(maxDiscoverySize)
}

func (dht *DHT) recv() {
	buf := make([]byte, 65535)
	for {
		select {
		case <-dht.ctx.Done():
			return
		default:
		}
		dht.listen.SetReadDeadline(time.Now().Add(time.Second))
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
			logging.Info("drop packet")
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
			if dht.tb.size < dht.minNodes {
				dht.tb.discovery(maxDiscoverySize)
			} else if dht.tx.size() == 0 {
				dht.tb.discovery(maxDiscoverySize)
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
			var req struct {
				data.Hdr
				Data struct {
					ID [20]byte `bencode:"id"`
				} `bencode:"a"`
			}
			err = bencode.Decode(buf, &req)
			if err != nil {
				return
			}
			if bytes.Equal(req.Data.ID[:], emptyHash[:]) {
				return
			}
			node = dht.tb.findID(req.Data.ID)
			if node == nil {
				node = newNode(dht, req.Data.ID, *addr.(*net.UDPAddr))
				dht.tb.add(node)
			}
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
