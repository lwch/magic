package dht

import "bytes"

type broadcastInfo struct {
	hash  hashType
	count int
}

type broadcast struct {
	list     []broadcastInfo
	idx      int
	maxCount int
}

func newBroadcast(maxCache, maxCount int) *broadcast {
	return &broadcast{
		list:     make([]broadcastInfo, maxCache),
		maxCount: maxCount,
	}
}

func (bc *broadcast) allow(hash hashType, n int) bool {
	for i := 0; i < len(bc.list); i++ {
		info := bc.list[i]
		if bytes.Equal(info.hash[:], hash[:]) {
			return info.count+n < bc.maxCount
		}
	}
	return true
}

func (bc *broadcast) broadcast(hash hashType, n int) int {
	defer func() {
		bc.idx++
	}()
	idx := bc.idx
	for i := 0; i < len(bc.list); i++ {
		info := &bc.list[i]
		if bytes.Equal(info.hash[:], hash[:]) {
			info.count += n
			return info.count
		}
	}
	if idx >= len(bc.list) {
		idx = idx % len(bc.list)
	}
	info := &bc.list[idx]
	copy(info.hash[:], hash[:])
	info.count += n
	return info.count
}
