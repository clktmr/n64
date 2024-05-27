// The CPU accesses RAM through a cache and in general assumes that there are no
// other readers or writers.  Since the stored value in the cache can divert
// from the stored value in RAM for a limited amount of time, we need to sync
// both before other comoponents are involved.
//
// All operations in this package refer to the data cache.  Instruction cache
// won't be affected.
package cpu

import "unsafe"

const CacheLineSize = 16
const cacheLineMask = ^(CacheLineSize - 1)

// Cache operations always affect a whole cache line.  To avoid invalidating
// unrelated data in a cache line, pad structs with CacheLinePad at the
// beginning and end.
type CacheLinePad struct{ _ [CacheLineSize]byte }

// Causes the cache to be written back to RAM.  Call this before requesting
// another component to read from this address range.  If the specified address
// is currently not cached, this is a no-op.
func Writeback(addr uintptr, length int)

// Causes the cache to be read from RAM before next access.  Call this before
// reading the address range if it was written by another component.  If the
// specified address is currently not cached, this is a no-op.
func Invalidate(addr uintptr, length int)

// A slice that is safe for cache ops.  It's start is aligned to CacheLineSize
// and the end is padded fill the cache line.  Note that using append() might
// corrupt the padding.
type paddedSlice []byte

// Wrapper around make() to create paddedSlice.
func MakePaddedSlice(size int) paddedSlice {
	buf := make([]byte, 0, size+CacheLineSize+CacheLineSize)
	addr := uintptr(unsafe.Pointer(unsafe.SliceData(buf)))
	align := CacheLineSize - int(addr%CacheLineSize)
	return buf[align : align+size]
}

// Returns true if p is safe for cache ops, i.e. padded and aligned to cache.
func IsPadded(p []byte) bool {
	pAddr := uintptr(unsafe.Pointer(unsafe.SliceData(p)))
	return pAddr%CacheLineSize == 0 && cap(p)-len(p) >= CacheLineSize-len(p)%CacheLineSize
}
