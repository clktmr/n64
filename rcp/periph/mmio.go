package periph

import (
	"embedded/mmio"
	"embedded/rtos"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/clktmr/n64/rcp/cpu"
)

// R32 represents an register on the PI external bus.
//   - 0x0500_0000 to 0x1fbf_ffff
//   - 0x1fd0_0000 to 0x7fff_ffff
//
// MMIO on the PI external bus has additional sync and aligment requirements.
// Further reading: https://n64brew.dev/wiki/Memory_map#Physical_Memory_Map_accesses
type R32[T mmio.T32] struct{ r uint32 }
type U32 struct{ R32[uint32] }

// Store writes the value to the register. If the PI bus is currently busy via
// MMIO or DMA the goroutine is parked until the value was written.
func (r *R32[T]) Store(val T) {
	bufid, p, done := getBuf()
	_ = p[3]
	p[0] = byte(val >> 24)
	p[1] = byte(val >> 16)
	p[2] = byte(val >> 8)
	p[3] = byte(val)
	vaddr := uintptr(unsafe.Pointer(r))
	dma(dmaJob{cpu.PhysicalAddress(vaddr), p[:], dmaStore, done})
	if !done.Wait(1 * time.Second) {
		panic("dma timeout")
	}
	putBuf(bufid)
}

// Load reads the value from the register. If the PI bus is currently busy via
// MMIO or DMA the goroutine is parked until the value was read.
func (r *R32[T]) Load() (v T) {
	bufid, p, done := getBuf()
	vaddr := uintptr(unsafe.Pointer(r))
	dma(dmaJob{cpu.PhysicalAddress(vaddr), p[:], dmaLoad, done})
	if !done.Wait(1 * time.Second) {
		panic("dma timeout")
	}
	v = T(p[0])<<24 | T(p[1])<<16 | T(p[2])<<8 | T(p[3])
	putBuf(bufid)
	return
}

// StoreSafe is the same as [R32.Store] but instead of parking the goroutine it
// will busywait until done, which makes it safe to use from interrupt.
//
//go:nosplit
func (r *R32[T]) StoreSafe(v T) {
	for !dmaActive.CompareAndSwap(false, true) {
		// wait
	}
	(*r32[T])(unsafe.Pointer(r)).Store(v)
	dmaActive.Store(false)
}

// LoadSafe is the same as [R32.Load] but instead of parking the goroutine it
// will busywait until done, which makes it safe to use from interrupt.
//
//go:nosplit
func (r *R32[T]) LoadSafe() (v T) {
	for !dmaActive.CompareAndSwap(false, true) {
		// wait
	}
	v = (*r32[T])(unsafe.Pointer(r)).Load()
	dmaActive.Store(false)
	return
}

// Addr returns the virtual address which is used to access the register.
func (r *R32[_]) Addr() uintptr {
	return uintptr(unsafe.Pointer(r))
}

var dmaBufPool [32]struct {
	buf  [4]byte
	done rtos.Cond
	used atomic.Bool
}

func getBuf() (int, []byte, *rtos.Cond) {
	for i := range dmaBufPool {
		b := &dmaBufPool[i]
		if b.used.CompareAndSwap(false, true) {
			return i, b.buf[:], &b.done
		}
	}

	var buf [4]byte
	return -1, buf[:], &rtos.Cond{}
}

func putBuf(i int) {
	if i < 0 {
		return
	}
	dmaBufPool[i].used.Store(false)
}

type u32 struct{ r32[uint32] }
type r32[T mmio.T32] struct{ r mmio.R32[T] }

//go:nosplit
func (r *r32[T]) Store(v T) {
	r.r.Store(v)
	for regs().status.Load()&(ioBusy) != 0 {
		// wait
	}
}

//go:nosplit
func (r *r32[T]) Load() T { return r.r.Load() }

//go:nosplit
func (r *r32[_]) Addr() uintptr { return r.r.Addr() }
