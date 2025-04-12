// MMIO on the PI external bus has additional sync and aligment requirements.
// Further reading: https://n64brew.dev/wiki/Memory_map#Physical_Memory_Map_accesses
package periph

import (
	"embedded/mmio"
	"embedded/rtos"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/clktmr/n64/rcp/cpu"
)

// Use for MMIO on the PI external bus:
// - 0x0500_0000 to 0x1fbf_ffff
// - 0x1fd0_0000 to 0x7fff_ffff

type U32 struct{ R32[uint32] }
type R32[T mmio.T32] struct{ r uint32 }

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

func (r *R32[T]) StoreSafe(v T) {
	for !dmaActive.CompareAndSwap(false, true) {
		// wait
	}
	(*r32[T])(unsafe.Pointer(r)).Store(v)
	dmaActive.Store(false)
}

func (r *R32[T]) LoadSafe() (v T) {
	for !dmaActive.CompareAndSwap(false, true) {
		// wait
	}
	v = (*r32[T])(unsafe.Pointer(r)).Load()
	dmaActive.Store(false)
	return
}

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
	for regs.status.Load()&(ioBusy) != 0 {
		// wait
	}
}

//go:nosplit
func (r *r32[T]) Load() T { return r.r.Load() }

//go:nosplit
func (r *r32[_]) Addr() uintptr { return r.r.Addr() }

// WriteIO copies slice p to PI bus address addr using PI bus IO.  Note that it
// needs to read from the PI bus if p's start or end aren't 4 byte aligned.
// This might lead to unexpected behaviour of write-only devices.
// TODO unexport, only for intr
//
//go:nosplit
func WriteIO(busAddr cpu.Addr, p []byte) {
	end := cpu.KSEG1 | uintptr(busAddr+cpu.Addr(len(p)+3))&^0x3
	shift := -(int(busAddr) & 0x3)
	endshift := ^(int(busAddr) + len(p) - 1) & 0x3

	busPtr := unsafe.Pointer(cpu.KSEG1 | uintptr(busAddr&^0x3))
	pPtr := unsafe.Pointer(unsafe.SliceData(p))
	pPtr = unsafe.Add(pPtr, shift)
	for uintptr(busPtr) < end {
		data, mask := uint32(0), uint32(0xffff_ffff)
		if shift != 0 { // first dword
			mask &= 0xffff_ffff >> ((-shift) << 3)
		}
		if end-uintptr(busPtr) == 4 && endshift != 0 { // last dword
			mask &= 0xffff_ffff << (endshift << 3)
		}
		if mask != 0xffff_ffff { // read data before writing
			data = (*u32)(busPtr).Load() &^ mask
		}
		if uintptr(pPtr)&0x3 == 0 {
			data |= *(*uint32)(pPtr) & mask
		} else { // unaligned access forbidden on mips
			p := *(*[4]byte)(pPtr)
			data |= (uint32(p[0])<<24 | uint32(p[1])<<16 | uint32(p[2])<<8 | uint32(p[3])) & mask
		}
		(*u32)(busPtr).Store(data)

		shift = 0
		pPtr = unsafe.Add(pPtr, 4)
		busPtr = unsafe.Add(busPtr, 4)
	}
}

// ReadIO copies from PI bus address addr to slice p using PI bus IO.
// TODO unexport, only for intr
//
//go:nosplit
func ReadIO(busAddr cpu.Addr, p []byte) {
	end := cpu.KSEG1 | uintptr(busAddr+cpu.Addr(len(p)+3))&^0x3
	shift := -(int(busAddr) & 0x3)

	busPtr := unsafe.Pointer(cpu.KSEG1 | uintptr(busAddr&^0x3))
	pPtr := unsafe.Pointer(unsafe.SliceData(p))
	pPtr = unsafe.Add(pPtr, shift)
	for uintptr(busPtr) < end {
		data := (*u32)(busPtr).Load()
		if uintptr(pPtr)&0x3 == 0 {
			*(*uint32)(pPtr) = data
		} else { // unaligned access forbidden on mips
			i, s := 0, (3+shift)<<3
			for i < min(len(p), shift+4) {
				p[i] = byte(data >> s)
				i, s = i+1, s-8
			}
			p = p[i:]
			shift = 0
		}

		pPtr = unsafe.Add(pPtr, 4)
		busPtr = unsafe.Add(busPtr, 4)
	}
}
