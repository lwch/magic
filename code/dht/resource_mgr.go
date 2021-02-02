package dht

import (
	"bytes"
	"context"
	"encoding/binary"
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

// MetaFile file info
type MetaFile struct {
	Path   []string `json:"path"`
	Length int      `json:"length"`
}

// MetaInfo meta info
type MetaInfo struct {
	Hash       string     `json:"hash"`
	Peer       string     `json:"peer"`
	Name       string     `json:"name"`
	Length     int        `json:"length"`
	MetaLength int        `json:"meta_length"`
	Files      []MetaFile `json:"files"`
}

type resMgr struct {
	dht   *DHT
	chReq chan resReq

	// runtime
	ctx    context.Context
	cancel context.CancelFunc
}

func newResMgr(dht *DHT) *resMgr {
	mgr := &resMgr{
		dht:   dht,
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
			go mgr.get(req, mgr.dht.Out)
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
	ret[25] = 0x10 // http://www.bittorrent.org/beps/bep_0010.html
	ret[27] = 1
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
	c.SetReadDeadline(time.Now().Add(time.Minute))
	var l uint32
	err := binary.Read(c, binary.BigEndian, &l)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("read header failed: %v", err)
	}
	payload := make([]byte, l)
	_, err = io.ReadFull(c, payload)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("read payload failed: %v", err)
	}
	switch l {
	case 0:
		return 0, 0, nil, nil
	case 1:
		return payload[0], 0, nil, nil
	default:
		return payload[0], payload[1], payload[2:], nil
	}
}

func sendExtHeader(c net.Conn) error {
	// http://www.bittorrent.org/beps/bep_0009.html
	var data struct {
		M struct {
			Action byte `bencode:"ut_metadata"`
		} `bencode:"m"`
	}
	data.M.Action = 1
	raw, err := bencode.Encode(data)
	if err != nil {
		return err
	}
	return sendMessage(c, extMsgID, 0, raw)
}

func readExtHeader(c net.Conn) (byte, int, int, error) {
	_, _, data, err := readMessage(c)
	if err != nil {
		return 0, 0, 0, err
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
		return 0, 0, 0, err
	}
	if hdr.Size == 0 {
		return byte(hdr.Data.Type), 0, 0, nil
	}
	pieces := float64(hdr.Size)/float64(blockSize) + .5
	if pieces < 1 {
		pieces = 1
	}
	return byte(hdr.Data.Type), hdr.Size, int(pieces), nil
}

// http://www.bittorrent.org/beps/bep_0009.html#request
func requestPiece(c net.Conn, metaData byte, n int) error {
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
	return sendMessage(c, extMsgID, metaData, data)
}

func (mgr *resMgr) get(r resReq, out chan MetaInfo) {
	c, err := net.DialTimeout("tcp", r.addr(), 5*time.Second)
	if err != nil {
		// logging.Error("*GET* connect failed" + r.errInfo(err))
		return
	}
	defer c.Close()
	_, err = c.Write(makeHandshake(r.id))
	if err != nil {
		// logging.Error("*GET* send handshake failed" + r.errInfo(err))
		return
	}
	err = readHandshake(c)
	if err != nil {
		// logging.Error("*GET* read handshake failed" + r.errInfo(err))
		return
	}
	err = sendExtHeader(c)
	if err != nil {
		// logging.Error("*GET* send ext header failed" + r.errInfo(err))
		return
	}
	metaData, metaSize, pieces, err := readExtHeader(c)
	if err != nil {
		// logging.Error("*GET* read ext header failed" + r.errInfo(err))
		return
	}
	logging.Info("*GET* resource %s from %s, pieces=%d, size=%d",
		r.id.String(), r.addr(), pieces, metaSize)
	for i := 0; i < pieces; i++ {
		err = requestPiece(c, metaData, i)
		if err != nil {
			logging.Error("*GET* send request piece %d failed"+r.errInfo(err), i)
			return
		}
	}
	pieceData := make([][]byte, pieces)
	totalLength := func() int {
		size := 0
		for i := 0; i < pieces; i++ {
			size += len(pieceData[i])
		}
		return size
	}
	for {
		msgID, _, data, err := readMessage(c)
		if err != nil {
			logging.Error("*GET* read peer data failed" + r.errInfo(err))
			return
		}
		if msgID != extMsgID {
			continue
		}
		buf := bytes.NewBuffer(data)
		dec := bencode.NewDecoder(buf)
		// http://www.bittorrent.org/beps/bep_0009.html#data
		var hdr struct {
			Type  byte `bencode:"msg_type"`
			Piece int  `bencode:"piece"`
			Size  int  `bencode:"total_size"`
		}
		err = dec.Decode(&hdr)
		if err != nil {
			logging.Error("*GET* decode data header failed" + r.errInfo(err))
			return
		}
		if hdr.Type != extData {
			continue
		}
		pieceData[hdr.Piece] = append(pieceData[hdr.Piece], buf.Bytes()...)
		if totalLength() >= metaSize {
			var files struct {
				PieceLength int    `bencode:"piece length"`
				Length      int    `bencode:"length"`
				Name        string `bencode:"name"`
				Files       []struct {
					Length int      `bencode:"length"`
					Path   []string `bencode:"path"`
				} `bencode:"files"`
			}
			err = bencode.Decode(bytes.Join(pieceData, nil), &files)
			if err != nil {
				logging.Error("*GET* decode data body failed, piece=%d"+r.errInfo(err), hdr.Piece)
				return
			}
			var list []MetaFile
			for _, file := range files.Files {
				list = append(list, MetaFile{
					Path:   file.Path,
					Length: file.Length,
				})
			}
			out <- MetaInfo{
				Hash:       r.id.String(),
				Peer:       r.addr(),
				Name:       files.Name,
				Length:     files.Length,
				MetaLength: metaSize,
				Files:      list,
			}
			return
		}
	}
}
