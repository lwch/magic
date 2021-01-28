package hashmap

import (
	"time"
)

const magic = 0xa5a5a5a5a5a5a5a5

// Map hashmap
type Map struct {
	data    SliceData
	retry   int
	scanQPS int
	timeout time.Duration
}

// New create hashmap
func New(data SliceData, size uint64, retry int, timeout time.Duration) *Map {
	mp := &Map{
		data:    data,
		retry:   retry,
		timeout: timeout,
	}
	mp.data.Make(size)
	return mp
}

// Size get hashmap size
func (mp *Map) Size() uint64 {
	return mp.data.Size()
}

// Set set key and value
func (mp *Map) Set(key interface{}, value interface{}) {
	h := mp.data.Hash(key)
	idx := h % mp.data.Cap()
	for i := 0; i < mp.retry+1; i++ {
		if mp.data.Empty(idx) {
			if mp.data.Set(idx, key, value, time.Now().Add(mp.timeout), false) {
				return
			}
		} else if mp.data.Timeout(idx) {
			mp.data.Reset(idx)
		}
		if mp.data.KeyEqual(idx, key) {
			if mp.data.Set(idx, key, value, time.Now().Add(mp.timeout), true) {
				return
			}
		}
		idx = (idx + magic) % mp.data.Cap()
	}
	mp.data.Resize(mp.data.Cap() << 1)
	mp.Set(key, value)
}

// Get get value
func (mp *Map) Get(key interface{}) interface{} {
	h := mp.data.Hash(key)
	idx := h % mp.data.Cap()
	for i := 0; i < mp.retry+1; i++ {
		if mp.data.Timeout(idx) && !mp.data.Empty(idx) {
			mp.data.Reset(idx)
		}
		if mp.data.KeyEqual(idx, key) {
			return mp.data.Get(idx)
		}
		idx = (idx + magic) % mp.data.Cap()
	}
	return nil
}

// Remove remove data
func (mp *Map) Remove(key interface{}) bool {
	h := mp.data.Hash(key)
	idx := h % mp.data.Cap()
	for i := 0; i < mp.retry+1; i++ {
		if mp.data.Timeout(idx) && !mp.data.Empty(idx) {
			mp.data.Reset(idx)
		}
		if mp.data.KeyEqual(idx, key) {
			mp.data.Reset(idx)
			return true
		}
		idx = (idx + magic) % mp.data.Cap()
	}
	return false
}

// Clear clear timeout data
func (mp *Map) Clear() {
	for i := uint64(0); i < mp.data.Cap(); i++ {
		if !mp.data.Empty(i) && mp.data.Timeout(i) {
			mp.data.Reset(i)
		}
	}
}

// Data get map data
func (mp *Map) Data() interface{} {
	return mp.data
}
