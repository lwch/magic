package dht

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/lwch/magic/code/data"
	"github.com/lwch/magic/code/logging"
)

const protocol = "BitTorrent protocol"

type resReq struct {
	id   hashType
	ip   net.IP
	port uint16
}

func (r resReq) addr() string {
	return fmt.Sprintf("%s:%d", r.ip.String(), r.port)
}

func (r resReq) errInfo(err error) string {
	return fmt.Sprintf("; id=%s, addr=%s, err=%v",
		r.id.String(), r.addr(), err)
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

// http://www.bittorrent.org/beps/bep_0003.html
func makeHandshake(hash hashType) []byte {
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
	if string(data[0:19]) != protocol {
		return fmt.Errorf("invalid protocol: %s", string(data[0:19]))
	}
	// http://www.bittorrent.org/beps/bep_0010.html
	if data[24]&0x10 == 0 {
		return errors.New("not support extended messaging")
	}
	return nil
}

func (mgr *resMgr) get(r resReq) {
	logging.Info("*GET* resource %s from %s", r.id.String(), r.addr())
	c, err := net.DialTimeout("tcp", r.addr(), 5*time.Second)
	if err != nil {
		logging.Error("*GET* connect failed" + r.errInfo(err))
		return
	}
	defer c.Close()
	_, err = c.Write(makeHandshake(r.id))
	if err != nil {
		logging.Error("*GET* send handshake failed" + r.errInfo(err))
		return
	}
	err = readHandshake(c)
	if err != nil {
		logging.Error("*GET* read handshake failed" + r.errInfo(err))
		return
	}
}
