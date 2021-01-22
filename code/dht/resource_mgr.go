package dht

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/lwch/bencode"
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
		if cnt > 0 {
			logging.Info("resInfo: %d links, avg scan count %d, max scan count %d", links, total/cnt, max)
		}
	}
	for {
		show()
		time.Sleep(time.Second)
	}
}

func (mgr *resMgr) getInfo() {
	get := func() {
		var wg sync.WaitGroup
		for i := 0; i < len(mgr.found); i++ {
			if bytes.Equal(mgr.found[i].hash[:], emptyHash[:]) {
				break
			}
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				mgr.get(mgr.found[i])
			}(i)
		}
		wg.Wait()
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
	if string(data[0:19]) != protocol {
		return fmt.Errorf("invalid protocol: %s", string(data[0:19]))
	}
	// http://www.bittorrent.org/beps/bep_0010.html
	if data[24]&0x10 == 0 {
		return errors.New("not support extended messaging")
	}
	return nil
}

func sendExtHeader(c net.Conn) error {
	// http://www.bittorrent.org/beps/bep_0009.html
	var data struct {
		M struct {
			Action int `bencode:"ut_metadata"`
		} `bencode:"m"`
	}
	data.M.Action = 3
	raw, err := bencode.Encode(data)
	if err != nil {
		return err
	}
	// http://www.bittorrent.org/beps/bep_0010.html
	raw = append([]byte{20, 0}, raw...)
	c.SetWriteDeadline(time.Now().Add(5 * time.Second))
	err = binary.Write(c, binary.BigEndian, uint32(len(raw)))
	if err != nil {
		return fmt.Errorf("sendExtHeader length failed: %v", err)
	}
	_, err = c.Write(raw)
	if err != nil {
		return fmt.Errorf("sendExtHeader data failed: %v", err)
	}
	return nil
}

func readPeerData(c net.Conn) (uint8, uint8, []byte, error) {
	// http://www.bittorrent.org/beps/bep_0010.html
	c.SetReadDeadline(time.Now().Add(10 * time.Second))
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
	return payload[0], payload[1], payload[2:], nil
}

func (mgr *resMgr) get(r foundRes) {
	hexID := fmt.Sprintf("%x", r.hash)
	if _, ok := mgr.info[hexID]; ok {
		return
	}
	addr := fmt.Sprintf("%s:%d", r.ip.String(), r.port)
	c, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", r.ip.String(), r.port), 5*time.Second)
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
	err = sendExtHeader(c)
	if err != nil {
		logging.Error("*GET* send ext header to %s failed, err=%v", addr, err)
		return
	}
	for {
		msgID, extID, data, err := readPeerData(c)
		if err != nil {
			logging.Error("*GET* read peer data to %s failed, err=%v", addr, err)
			return
		}
		logging.Info("msg_id=%d, ext_id=%d", msgID, extID)
		logging.Info("read_data: %s", hex.Dump(data))
	}
}
