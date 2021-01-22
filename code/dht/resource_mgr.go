package dht

import (
	"bytes"
	"net"
	"sort"
	"time"

	"github.com/lwch/magic/code/logging"
)

type res struct {
	hash [20]byte
	cnt  int
}

type foundRes struct {
	hash [20]byte
	ip   net.IP
	port uint16
}

type resMgr struct {
	list     []res
	foundIdx int
	found    []foundRes
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
	// list not full
	size := mgr.size
	if size < len(mgr.list) {
		mgr.list[size].hash = hash
		mgr.list[size].cnt = 1
		mgr.size++
		return
	}
	// exists
	for i := 0; i < len(mgr.list); i++ {
		r := &mgr.list[i]
		if bytes.Equal(r.hash[:], hash[:]) {
			r.cnt++
			return
		}
	}
	// minimum elimination
	sort.Slice(mgr.list, func(i, j int) bool {
		return mgr.list[i].cnt < mgr.list[j].cnt
	})
	mgr.list[0].hash = hash
	mgr.list[0].cnt = 1
}

func (mgr *resMgr) markFound(hash [20]byte, ip net.IP, port uint16) {
	idx := mgr.foundIdx % mgr.maxScan
	mgr.found[idx] = foundRes{
		hash: hash,
		ip:   ip,
		port: port,
	}
	mgr.foundIdx++
}

func (mgr *resMgr) print() {
	show := func() {
		for i := 0; i < len(mgr.found); i++ {
			if bytes.Equal(mgr.found[i].hash[:], emptyHash[:]) {
				break
			}
			res := mgr.found[i]
			logging.Info("found res: %x, ip=%s, port=%d", res.hash, res.ip.String(), res.port)
		}
	}
	for {
		if mgr.foundIdx > 0 {
			show()
		}
		time.Sleep(time.Second)
	}
}
