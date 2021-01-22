package dht

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"sort"
	"time"

	"github.com/lwch/magic/code/data"
	"github.com/lwch/magic/code/logging"
)

const protocol = "BitTorrent protocol"

type res struct {
	hash [20]byte
	cnt  int
}

type foundRes struct {
	hash [20]byte
	ip   net.IP
	port uint16
}

type metaInfo struct {
	hash [20]byte
	ip   net.IP
	port uint16
	name string
	size uint64
}

type resMgr struct {
	list     []res
	foundIdx int
	found    []foundRes
	info     map[string]metaInfo
	size     int
	maxScan  int
}

func newResMgr(maxRes, maxScan int) *resMgr {
	mgr := &resMgr{
		list:    make([]res, maxRes),
		found:   make([]foundRes, maxRes),
		maxScan: maxScan,
	}
	go mgr.print()
	go mgr.getInfo()
	return mgr
}

func (mgr *resMgr) allowScan(hash [20]byte) bool {
	for i := 0; i < len(mgr.found); i++ {
		if bytes.Equal(mgr.found[i].hash[:], emptyHash[:]) {
			break
		}
		if bytes.Equal(mgr.found[i].hash[:], hash[:]) {
			return false
		}
	}
	for i := 0; i < len(mgr.list); i++ {
		r := mgr.list[i]
		if bytes.Equal(r.hash[:], hash[:]) {
			return r.cnt < mgr.maxScan
		}
	}
	return true
}

func (mgr *resMgr) scan(hash [20]byte) {
	// exists
	for i := 0; i < len(mgr.list); i++ {
		r := &mgr.list[i]
		if bytes.Equal(r.hash[:], hash[:]) {
			r.cnt++
			return
		}
	}
	// list not full
	size := mgr.size
	if size < len(mgr.list) {
		mgr.list[size].hash = hash
		mgr.list[size].cnt = 1
		mgr.size++
		return
	}
	// maximum elimination
	sort.Slice(mgr.list, func(i, j int) bool {
		return mgr.list[i].cnt > mgr.list[j].cnt
	})
	mgr.list[0].hash = hash
	mgr.list[0].cnt = 1
}

func (mgr *resMgr) markFound(hash [20]byte, ip net.IP, port uint16) {
	idx := mgr.foundIdx % len(mgr.found)
	mgr.found[idx] = foundRes{
		hash: hash,
		ip:   ip,
		port: port,
	}
	mgr.foundIdx++
}

func (mgr *resMgr) print() {
	show := func() {
		links := 0
		for i := 0; i < len(mgr.found); i++ {
			if bytes.Equal(mgr.found[i].hash[:], emptyHash[:]) {
				break
			}
			res := mgr.found[i]
			logging.Info("resource: %x in %s:%d", res.hash, res.ip.String(), res.port)
			links++
		}
		var cnt int
		var total int
		var max int
		for i := 0; i < len(mgr.list); i++ {
			res := mgr.list[i]
			if bytes.Equal(res.hash[:], emptyHash[:]) {
				break
			}
			if res.cnt > max {
				max = res.cnt
			}
			total += res.cnt
			cnt++
		}
		logging.Info("resInfo: %d links, avg scan count %d, max scan count %d", links, total/cnt, max)
	}
	for {
		show()
		time.Sleep(time.Second)
	}
}

func (mgr *resMgr) getInfo() {
	get := func() {
		for i := 0; i < len(mgr.found); i++ {
			if bytes.Equal(mgr.found[i].hash[:], emptyHash[:]) {
				break
			}
			mgr.get(mgr.found[i])
		}
	}
	for {
		if mgr.foundIdx > 0 {
			get()
		}
		time.Sleep(time.Second)
	}
}

// http://www.bittorrent.org/beps/bep_0003.html
func makeHandshake(hash [20]byte) []byte {
	ret := make([]byte, 68)
	ret[0] = 19
	copy(ret[1:], protocol)
	// 20:28 is reserved
	copy(ret[28:], hash[:])
	id := data.RandID()
	copy(ret[48:], id[:])
	return ret
}

func readHandshake(c net.Conn) error {
	var l [1]byte
	c.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, err := c.Read(l[:])
	if err != nil {
		return err
	}
	data := make([]byte, l[0]+48) // same as handshake request
	_, err = io.ReadFull(c, data)
	if err != nil {
		return err
	}
	if string(data[1:20]) != protocol {
		return errors.New("invalid protocol")
	}
	logging.Info("info: %s", hex.Dump(data[20:28]))
	return nil
}

func (mgr *resMgr) get(r foundRes) {
	hexID := fmt.Sprintf("%x", r.hash)
	if _, ok := mgr.info[hexID]; ok {
		return
	}
	addr := fmt.Sprintf("%s:%d", r.ip.String(), r.port)
	c, err := net.DialTCP("tcp", nil, &net.TCPAddr{
		IP:   r.ip,
		Port: int(r.port),
	})
	if err != nil {
		logging.Error("*GET* connect to %s failed, err=%v", addr, err)
		return
	}
	defer c.Close()
	_, err = c.Write(makeHandshake(r.hash))
	if err != nil {
		logging.Error("*GET* write handshake to %s failed, err=%v", addr, err)
		return
	}
	err = readHandshake(c)
	if err != nil {
		logging.Error("*GET* read handshake length to %s failed, err=%v", addr, err)
		return
	}
}
