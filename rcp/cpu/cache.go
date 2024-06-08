// The CPU accesses RAM through a cache and in general assumes that there are no
// other readers or writers.  Since the stored value in the cache can divert
// from the stored value in RAM for a limited amount of time, we need to sync
// both before other comoponents are involved.
//
// All operations in this package refer to the data cache.  Instruction cache
// won't be affected.
package cpu

import (
	"n64/debug"
	"unsafe"
)

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
// the address range is to be written by another component.  If the specified
// address is currently not cached, this is a no-op.
func Invalidate(addr uintptr, length int)

// A slice that is safe for cache ops.  It's start is aligned to CacheLineSize
// and the end is padded to fill the cache line.  Note that using append() might
// corrupt the padding.
// Aligning the slice start to CacheLineSize has the advantage that runtime
// validation is possible, see IsPadded().
func MakePaddedSlice[T any](size int) []T {
	var t T
	cls := CacheLineSize / int(unsafe.Sizeof(t))
	buf := make([]T, 0, cls+size+cls)
	addr := uintptr(unsafe.Pointer(unsafe.SliceData(buf)))
	shift := (CacheLineSize - int(addr)%CacheLineSize) / int(unsafe.Sizeof(t))
	return buf[shift : shift+size]
}

// Ensure a slice is padded.  Might copy the slice if necessary
func PaddedSlice[T any](slice []T) []T {
	if IsPadded(slice) == false {
		buf := MakePaddedSlice[T](len(slice))
		copy(buf, slice)
		return buf
	}
	return slice
}

// Same as MakePaddedSlice with extra alignment requirements.
func MakePaddedSliceAligned[T any](size int, align uintptr) []T {
	var t T
	if align <= CacheLineSize || align <= unsafe.Alignof(t) {
		return MakePaddedSlice[T](size)
	}

	buf := MakePaddedSlice[T](size + int(align/unsafe.Sizeof(t)))
	addr := uintptr(unsafe.Pointer(unsafe.SliceData(buf)))
	shift := (align - addr%align) / unsafe.Sizeof(t)
	return buf[shift : shift+uintptr(size)]
}

// Returns true if p is safe for cache ops, i.e. padded and aligned to cache.
func IsPadded[T any](p []T) bool {
	var t T
	cls := CacheLineSize / int(unsafe.Sizeof(t))

	addr := uintptr(unsafe.Pointer(unsafe.SliceData(p)))
	return addr%CacheLineSize == 0 && cap(p)-len(p) >= cls-len(p)%cls
}

func WritebackSlice[T any](buf []T) {
	debug.Assert(IsPadded(buf), "unpadded cache writeback")

	var t T
	addr := uintptr(unsafe.Pointer(unsafe.SliceData(buf)))
	Writeback(addr, len(buf)*int(unsafe.Sizeof(t)))
}

func InvalidateSlice[T any](buf []T) {
	debug.Assert(IsPadded(buf), "unpadded cache invalidate")

	var t T
	addr := uintptr(unsafe.Pointer(unsafe.SliceData(buf)))
	Invalidate(addr, len(buf)*int(unsafe.Sizeof(t)))
}
