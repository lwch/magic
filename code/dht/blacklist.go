package dht

import (
	"net"
	"sync"
	"time"

	"github.com/lwch/magic/code/logging"
)

// TODO: timeout
type blacklist struct {
	sync.RWMutex
	addr map[string]bool
	id   map[string]bool
}

func newBlackList() *blacklist {
	bl := &blacklist{
		addr: make(map[string]bool),
		id:   make(map[string]bool),
	}
	go bl.print()
	return bl
}

func (bl *blacklist) blockAddr(addr net.Addr) {
	bl.Lock()
	defer bl.Unlock()
	bl.addr[addr.String()] = true
}

func (bl *blacklist) blockID(id hashType) {
	bl.Lock()
	defer bl.Unlock()
	bl.id[id.String()] = true
}

func (bl *blacklist) isBlockAddr(addr net.Addr) bool {
	bl.RLock()
	defer bl.RUnlock()
	return bl.addr[addr.String()]
}

func (bl *blacklist) isBlockID(id hashType) bool {
	bl.RLock()
	defer bl.RUnlock()
	return bl.id[id.String()]
}

func (bl *blacklist) print() {
	tk := time.NewTicker(time.Second)
	for {
		<-tk.C
		logging.Info("blacklist: %d ip blocked, %d id blocked", len(bl.addr), len(bl.id))
	}
}
