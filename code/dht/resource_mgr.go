package dht

import (
	"context"
	"net"

	"github.com/lwch/magic/code/logging"
)

const protocol = "BitTorrent protocol"

type resReq struct {
	id   hashType
	ip   net.IP
	port uint16
}

type resMgr struct {
	chReq chan resReq

	// runtime
	ctx    context.Context
	cancel context.CancelFunc
}

func newResMgr() *resMgr {
	mgr := &resMgr{
		chReq: make(chan resReq, 100),
	}
	mgr.ctx, mgr.cancel = context.WithCancel(context.Background())
	go mgr.loopGet()
	return mgr
}

func (mgr *resMgr) push(r resReq) {
	mgr.chReq <- r
}

func (mgr *resMgr) close() {
	mgr.cancel()
}

func (mgr *resMgr) loopGet() {
	for {
		select {
		case req := <-mgr.chReq:
			go mgr.get(req)
		case <-mgr.ctx.Done():
			return
		}
	}
}

func (mgr *resMgr) get(r resReq) {
	logging.Info("get resource from %s:%d", r.id.String(), r.ip.String(), r.port)
}
