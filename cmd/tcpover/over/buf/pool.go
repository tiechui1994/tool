package buf

import (
	"sync"
)

func createAllocFunc(size int32) func() interface{} {
	return func() interface{} {
		return make([]byte, size)
	}
}

const (
	numPools  = 4
	sizeMulti = 2
)

var (
	poolStu struct {
		once sync.Once
		pool     [numPools]sync.Pool
		poolSize [numPools]int32
	}
)

func init() {
	poolStu.once.Do(initPool)
}

func initPool()  {
	size := int32(2048)
	for i := 0; i < numPools; i++ {
		poolStu.pool[i] = sync.Pool{
			New: createAllocFunc(size),
		}
		poolStu.poolSize[i] = size
		size *= sizeMulti
	}
}

// GetPool returns a sync.Pool that generates bytes array with at least the given size.
// It may return nil if no such pool exists.
func GetPool(size int32) *sync.Pool {
	poolStu.once.Do(initPool)
	for idx, ps := range poolStu.poolSize {
		if size <= ps {
			return &poolStu.pool[idx]
		}
	}
	return nil
}

// Alloc returns a byte slice with at least the given size. Minimum size of returned slice is 2048.
func Alloc(size int32) []byte {
	pool := GetPool(size)
	if pool != nil {
		return pool.Get().([]byte)
	}
	return make([]byte, size)
}

// Free puts a byte slice into the internal pool.
func Free(b []byte) {
	size := int32(cap(b))
	b = b[0:cap(b)]
	for i := numPools - 1; i >= 0; i-- {
		if size >= poolStu.poolSize[i] {
			poolStu.pool[i].Put(b) // nolint: staticcheck
			return
		}
	}
}
