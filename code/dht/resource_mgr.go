package dht

import (
	"bytes"
	"sort"
)

type res struct {
	hash [20]byte
	cnt  int
}

type resMgr struct {
	list    []res
	size    int
	maxScan int
}

func newResMgr(maxRes, maxScan int) *resMgr {
	return &resMgr{
		list:    make([]res, maxRes),
		maxScan: maxScan,
	}
}

func (mgr *resMgr) allowScan(hash [20]byte) bool {
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
