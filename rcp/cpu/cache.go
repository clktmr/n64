// Package cpu provides helpers to share memory between the CPU and other
// hardware components.
//
// To share memory managed by the Go runtime, the following requirements must be
// met:
//   - Cache: Cached values must be written or invalidated before memory is
//     handed over to another component.
//   - Alignment: Most components require stricter memory alignment than Go has.
//   - Memory Location: The shared memory must be allocated on the heap, since
//     the stack might move if it has to grow.
//   - Memory Layout: If sharing a pointer to a struct, include structs.HostLayout
//     in the type definition. Go itself makes no guarantees regarding a structs
//     memory layout.
//
// All cache operations in this package refer to the data cache. Instruction
// cache won't be affected.
package cpu

import (
	"structs"
	"unsafe"

	"github.com/clktmr/n64/debug"
)

const CacheLineSize = 16
const cacheLineMask = ^(CacheLineSize - 1)

// Cache operations always affect a whole cache line. To avoid invalidating
// unrelated data in a cache line, pad structs with CacheLinePad at the
// beginning and end.
type CacheLinePad struct{ _ [CacheLineSize]byte }

// Causes the cache to be written back to RAM. Call this before requesting
// another component to read from this address range. If the specified address
// is currently not cached, this is a no-op.
func Writeback(addr uintptr, length int)

// Causes the cache to be read from RAM before next access. Call this before
// the address range is to be written by another component. If the specified
// address is currently not cached, this is a no-op.
func Invalidate(addr uintptr, length int)

// Cached is a datatype that provides cache operations.
type Cached interface {
	// Writeback writes all cached pixels to memory. Call before passing an
	// object that was modified by the CPU to another hardware component.
	Writeback()

	// Invalidate discards all currently cached pixel values. Call before
	// reading a texture that was modified by the CPU to another hardware
	// component.
	Invalidate()
}

// Only types with CacheLineSize%unsafe.Sizeof(T) == 0
type Paddable interface {
	~uint8 | ~uint16 | ~uint32 | ~uint64 | ~int8 | ~int16 | ~int32 | ~int64
}

// A slice that is safe for cache ops. It's start is aligned to CacheLineSize
// and the end is padded to fill the cache line. Note that using append() might
// corrupt the padding.
// Aligning the slice start to CacheLineSize has the advantage that runtime
// validation is possible, see IsPadded().
func MakePaddedSlice[T Paddable](size int) []T {
	var t T
	cls := CacheLineSize / int(unsafe.Sizeof(t))
	buf := make([]T, 0, cls+size+cls)
	addr := uintptr(unsafe.Pointer(unsafe.SliceData(buf)))
	shift := (CacheLineSize - int(addr)%CacheLineSize) / int(unsafe.Sizeof(t))
	return buf[shift : shift+size]
}

// Ensure a slice is padded. Might copy the slice if necessary
func CopyPaddedSlice[T Paddable](slice []T) []T {
	if IsPadded(slice) == false {
		buf := MakePaddedSlice[T](len(slice))
		copy(buf, slice)
		return buf
	}
	return slice
}

// Returns the size of necessary cacheline pads to get a padded slice, i.e. the
// slice buf[head:tail] is the part of buf which is safe for cache ops.
func Pads[T Paddable](buf []T) (int, int) {
	// TODO Review
	var t T
	start := uintptr(unsafe.Pointer(unsafe.SliceData(buf)))
	end := start + uintptr(cap(buf))*unsafe.Sizeof(t)
	head := ((CacheLineSize - int(start)%CacheLineSize) % CacheLineSize) / int(unsafe.Sizeof(t))
	tail := int(end) % CacheLineSize / int(unsafe.Sizeof(t))
	if head+tail >= cap(buf) {
		return 0, 0
	}
	tail = max(0, tail-(cap(buf)-len(buf)))
	head = min(head, len(buf)-tail)
	return head, len(buf) - tail
}

// Add padding to a given slice by shrinking it. Returns the number of
// discarded elements at the beginnning of the slice as second return value.
func PadSlice[T Paddable](buf []T) ([]T, int, int) {
	head, tail := Pads(buf)
	return buf[head:tail], head, len(buf) - tail
}

// Same as MakePaddedSlice with extra alignment requirements.
func MakePaddedSliceAligned[T Paddable](size int, align uintptr) []T {
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
func IsPadded[T Paddable](p []T) bool {
	if len(p) == 0 {
		return true
	}

	var t T
	cls := CacheLineSize / int(unsafe.Sizeof(t))

	addr := uintptr(unsafe.Pointer(unsafe.SliceData(p)))
	return addr%CacheLineSize == 0 && cap(p)-len(p) >= cap(p)%cls
}

// Like [Writeback], but for a padded slice.
func WritebackSlice[T Paddable](buf []T) {
	debug.Assert(IsPadded(buf), "unpadded cache writeback")

	var t T
	addr := uintptr(unsafe.Pointer(unsafe.SliceData(buf)))
	Writeback(addr, len(buf)*int(unsafe.Sizeof(t)))
}

// Like [Invalidate], but for a padded slice.
func InvalidateSlice[T Paddable](buf []T) {
	debug.Assert(IsPadded(buf), "unpadded cache invalidate")

	var t T
	addr := uintptr(unsafe.Pointer(unsafe.SliceData(buf)))
	Invalidate(addr, len(buf)*int(unsafe.Sizeof(t)))
}

// Uncached returns a pointer to p with caching disabled. The returned pointer
// is only valid as long as p exists, as it doesn't prevent the object from
// being garbage collected.
func Uncached[T any](p *T) *T {
	ptr := uintptr(unsafe.Pointer(p)) | KSEG1
	return (*T)(unsafe.Pointer(ptr))
}

// MMIO returns a pointer to an object that is memory mapped.
func MMIO[T any](ptr Addr) *T {
	if ptr < 0x3f00000 {
		panic("mmio in ram")
	}
	return (*T)(unsafe.Pointer(uintptr(ptr) | KSEG1))
}

// UncachedSlice returns a new slice with the same underlying data as s, with
// caching disabled. The returned slice is only valid as long as s exists, as it
// doesn't prevent the underlying array from being garbage collected.
func UncachedSlice[T any](s []T) []T {
	ptr := uintptr(unsafe.Pointer(unsafe.SliceData(s))) | KSEG1
	return unsafe.Slice((*T)(unsafe.Pointer(ptr)), len(s))
}

type Alignment interface {
	Align16 | Align64
}
type Align16 = [16]byte
type Align64 = [64]byte

// Padded embeds T with cacheline pads around it to make it safe for cache
// operations. It also guarantees the specified memory alignment for T.
//
// T must not hold any heap pointers.
type Padded[T any, A Alignment] struct {
	_   structs.HostLayout
	pad A
	val T
	_   CacheLinePad
}

func NewPadded[T any, A Alignment]() *Padded[T, A] {
	a := &Padded[T, A]{}
	escape(&a)
	return a
}

func (p *Padded[T, A]) Value() *T {
	size := unsafe.Sizeof(p.pad)
	align := size - uintptr(unsafe.Pointer(&p.pad[0]))&(size-1)
	return (*T)(unsafe.Pointer(&p.pad[align&(size-1)]))
}

func (p *Padded[T, A]) Writeback() {
	Writeback(uintptr(unsafe.Pointer(p.Value())), int(unsafe.Sizeof(p.val)))
}

func (p *Padded[T, A]) Invalidate() {
	Invalidate(uintptr(unsafe.Pointer(p.Value())), int(unsafe.Sizeof(p.val)))
}

// escape ensures that v escapes to heap. This is used to avoid	allocation on
// stack, because the stack might be moved when it grows.
func escape(v any)
