package dht

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/lwch/bencode"
	"github.com/lwch/magic/code/data"
	"github.com/lwch/magic/code/logging"
)

const protocol = "BitTorrent protocol"

// http://www.bittorrent.org/beps/bep_0010.html
const extMsgID = byte(20)
const extRequest = byte(0)
const extData = byte(1)
const extReject = byte(2)

// http://www.bittorrent.org/beps/bep_0009.html#metadata
const blockSize = 16 * 1024

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

func (r resReq) logInfo() string {
	return fmt.Sprintf("; id=%s, addr=%s",
		r.id.String(), r.addr())
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

// http://www.bittorrent.org/beps/bep_0010.html
func sendMessage(c net.Conn, msgID, extID byte, payload []byte) error {
	c.SetWriteDeadline(time.Now().Add(10 * time.Second))
	err := binary.Write(c, binary.BigEndian, uint32(len(payload)+2))
	if err != nil {
		return err
	}
	_, err = c.Write(append([]byte{msgID, extID}, payload...))
	if err != nil {
		return err
	}
	return nil
}

// http://www.bittorrent.org/beps/bep_0010.html
func readMessage(c net.Conn) (uint8, uint8, []byte, error) {
	c.SetReadDeadline(time.Now().Add(10 * time.Second))
	var l uint32
	err := binary.Read(c, binary.BigEndian, &l)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("read header failed: %v", err)
	}
	if l < 2 {
		return 0, 0, nil, errors.New("invalid data of length")
	}
	payload := make([]byte, l)
	_, err = io.ReadFull(c, payload)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("read payload failed: %v", err)
	}
	return payload[0], payload[1], payload[2:], nil
}

func sendExtHeader(c net.Conn) error {
	// http://www.bittorrent.org/beps/bep_0009.html
	var data struct {
		M struct {
			Action byte `bencode:"ut_metadata"`
		} `bencode:"m"`
	}
	data.M.Action = extRequest
	raw, err := bencode.Encode(data)
	if err != nil {
		return err
	}
	return sendMessage(c, extMsgID, 0, raw)
}

func readExtHeader(c net.Conn) (int, error) {
	_, _, data, err := readMessage(c)
	if err != nil {
		return 0, err
	}
	// http://www.bittorrent.org/beps/bep_0010.html
	var hdr struct {
		Port    uint16 `bencode:"p"`
		Version string `bencode:"v"`
		IP      string `bencode:"yourip"`
		Data    struct {
			Type int `bencode:"ut_metadata"` // http://www.bittorrent.org/beps/bep_0009.html
		} `bencode:"m"`
		Size int `bencode:"metadata_size"`
	}
	err = bencode.Decode(data, &hdr)
	if err != nil {
		return 0, err
	}
	pieces := hdr.Size / blockSize
	if pieces == 0 {
		pieces = 1
	}
	logging.Info("ut_metadata=%d", hdr.Data.Type)
	return pieces, nil
}

// http://www.bittorrent.org/beps/bep_0009.html#request
func requestPiece(c net.Conn, n int) error {
	var req struct {
		Type  byte `bencode:"msg_type"`
		Piece int  `bencode:"piece"`
	}
	req.Type = extRequest
	req.Piece = n
	data, err := bencode.Encode(req)
	if err != nil {
		logging.Error("build request packet failed, addr=%s, err=%v",
			c.RemoteAddr().String(), err)
		return err
	}
	logging.Info("piece: %s", hex.Dump(data))
	return sendMessage(c, extMsgID, extData, data)
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
	err = sendExtHeader(c)
	if err != nil {
		logging.Error("*GET* send ext header failed" + r.errInfo(err))
		return
	}
	pieces, err := readExtHeader(c)
	if err != nil {
		logging.Error("*GET* read ext header failed" + r.errInfo(err))
		return
	}
	for i := 0; i < pieces; i++ {
		err = requestPiece(c, i)
		if err != nil {
			logging.Error("*GET* send request piece %d failed"+r.errInfo(err), i)
			return
		}
	}
	logging.Info("*GET* request pieces done, pieces=%d"+r.logInfo(), pieces)
	for {
		msgID, extID, data, err := readMessage(c)
		if err != nil {
			logging.Error("*GET* read peer data failed" + r.errInfo(err))
			return
		}
		logging.Info("msg_id=%d, ext_id=%d", msgID, extID)
		logging.Info("read_data: %s", hex.Dump(data))
	}
}
