package dht

import (
	"container/list"
	"crypto/md5"
	"encoding/binary"
	"sync"
	"time"

	"github.com/lwch/magic/code/data"
	"github.com/lwch/magic/code/logging"
)

const txBucketSize = 32

type tx struct {
	id       string       // transaction id
	hash     hashType     // get_peers.info_hash
	remote   hashType     // find_node.target
	t        data.ReqType // request type
	deadline time.Time
}

type txMgr struct {
	sync.RWMutex
	list    [txBucketSize]*list.List
	count   int
	timeout time.Duration
}

func newTXMgr(timeout time.Duration) *txMgr {
	mgr := &txMgr{timeout: timeout}
	for i := 0; i < txBucketSize; i++ {
		mgr.list[i] = list.New()
	}
	go mgr.print()
	return mgr
}

func (mgr *txMgr) close() {
}

func (mgr *txMgr) size() int {
	return mgr.count
}

func txHash(tx string) uint {
	enc := md5.Sum([]byte(tx))
	a := binary.BigEndian.Uint64(enc[0:])
	b := binary.BigEndian.Uint64(enc[8:])
	return uint(a + b)
}

func (mgr *txMgr) add(id string, t data.ReqType, hash hashType, remote hashType) {
	list := mgr.list[txHash(id)%txBucketSize]
	mgr.clearTimeout(list)
	if list.Len() >= 1000 {
		mgr.Lock()
		list.Remove(list.Front())
		mgr.count--
		mgr.Unlock()
	}
	mgr.Lock()
	list.PushBack(tx{
		id:       id,
		hash:     hash,
		remote:   remote,
		t:        t,
		deadline: time.Now().Add(mgr.timeout),
	})
	mgr.count++
	mgr.Unlock()
}

func (mgr *txMgr) find(id string) *tx {
	l := mgr.list[txHash(id)%txBucketSize]
	mgr.clearTimeout(l)
	var node *list.Element
	mgr.RLock()
	for node = l.Back(); node != nil; node = node.Prev() {
		if node.Value.(tx).id == id {
			break
		}
	}
	mgr.RUnlock()
	if node != nil {
		mgr.Lock()
		tx := l.Remove(node).(tx)
		mgr.count--
		mgr.Unlock()
		return &tx
	}
	return nil
}

func (mgr *txMgr) clearTimeout(list *list.List) {
	mgr.Lock()
	for node := list.Front(); node != nil; node = node.Next() {
		if time.Now().After(node.Value.(tx).deadline) {
			list.Remove(node)
		}
		break
	}
	mgr.Unlock()
}

func (mgr *txMgr) print() {
	print := func() {
		size := make([]int, txBucketSize)
		for i := 0; i < txBucketSize; i++ {
			size[i] = mgr.list[i].Len()
		}
		logging.Info("tx: %v", size)
	}
	for {
		print()
		time.Sleep(time.Second)
	}
}
