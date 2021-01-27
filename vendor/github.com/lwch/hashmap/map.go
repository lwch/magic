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
func New(data SliceData, size uint64, retry, scanQPS int, timeout time.Duration) *Map {
	mp := &Map{
		data:    data,
		retry:   retry,
		scanQPS: scanQPS,
		timeout: timeout,
	}
	mp.data.Make(size)
	go mp.expire()
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
			mp.data.Set(idx, key, value, time.Now().Add(mp.timeout), false)
			return
		}
		if mp.data.KeyEqual(idx, key) {
			mp.data.Set(idx, key, value, time.Now().Add(mp.timeout), true)
			return
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
		if mp.data.KeyEqual(idx, key) {
			return mp.data.Get(idx)
		}
		idx = (idx + magic) % mp.data.Cap()
	}
	return nil
}

// Remove remove data
func (mp *Map) Remove(key interface{}) {
	h := mp.data.Hash(key)
	idx := h % mp.data.Cap()
	for i := 0; i < mp.retry+1; i++ {
		if mp.data.KeyEqual(idx, key) {
			mp.data.Reset(idx)
		}
		idx = (idx + magic) % mp.data.Cap()
	}
}

func (mp *Map) expire() {
	for {
		dt := time.Now()
		cnt := 0
		for i := uint64(0); i < mp.data.Cap(); i++ {
			if !mp.data.Empty(i) && mp.data.Timeout(i) {
				mp.data.Reset(i)
			}
			cnt++
			now := time.Now()
			if now.Unix() == dt.Unix() {
				leftTime := 999999999 - now.Nanosecond()
				left := mp.scanQPS - cnt
				if left <= 0 {
					time.Sleep(time.Duration(leftTime))
				} else {
					time.Sleep(time.Duration(leftTime / left))
				}
			} else if now.Unix() > dt.Unix() {
				dt = now
				cnt = 1
			}
		}
	}
}

// Data get map data
func (mp *Map) Data() interface{} {
	return mp.data
}
