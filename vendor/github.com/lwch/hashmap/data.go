package hashmap

import "time"

// SliceData hashmap data
type SliceData interface {
	Make(uint64)
	Resize(uint64)
	Size() uint64
	Cap() uint64
	Hash(key interface{}) uint64
	KeyEqual(idx uint64, key interface{}) bool
	Empty(idx uint64) bool
	Set(idx uint64, key, value interface{}, deadtime time.Time, update bool) bool
	Get(idx uint64) interface{}
	Reset(idx uint64)
	Timeout(idx uint64) bool
}
