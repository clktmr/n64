// MMIO on the PI external bus has additional sync and aligment requirements.
// Further reading: https://n64brew.dev/wiki/Memory_map#Physical_Memory_Map_accesses
package periph

import (
	"embedded/mmio"
	"unsafe"

	"github.com/clktmr/n64/rcp/cpu"
)

// Use for MMIO on the PI external bus:
// - 0x0500_0000 to 0x1fbf_ffff
// - 0x1fd0_0000 to 0x7fff_ffff

type U32 struct{ R32[uint32] }
type R32[T mmio.T32] struct{ r uint32 }

func (r *R32[T]) Store(val T) {
	p, _ := getBuf() // TODO dont waste a whole buffer
	_ = p[3]
	p[0] = byte(val >> 24)
	p[1] = byte(val >> 16)
	p[2] = byte(val >> 8)
	p[3] = byte(val)
	vaddr := uintptr(unsafe.Pointer(r))
	dma(dmaJob{cpu.PhysicalAddress(vaddr), p[:4], dmaStore, nil})
}

func (r *R32[T]) Load() (v T) {
	p, done := getBuf() // TODO dont waste a whole buffer
	vaddr := uintptr(unsafe.Pointer(r))
	jid := dma(dmaJob{cpu.PhysicalAddress(vaddr), p[:4], dmaLoad, nil})
	flush(jid, done)
	return T(p[0])<<24 | T(p[1])<<16 | T(p[2])<<8 | T(p[3])
}

func (r *R32[_]) Addr() uintptr {
	return uintptr(unsafe.Pointer(r))
}

type U32Safe struct{ R32Safe[uint32] }
type R32Safe[T mmio.T32] struct{ r uint32 }

func (r *R32Safe[T]) Store(v T) {
	for !dmaActive.CompareAndSwap(false, true) {
		// wait
	}
	(*r32[T])(unsafe.Pointer(r)).Store(v)
	dmaActive.Store(false)
}

func (r *R32Safe[T]) Load() (v T) {
	for !dmaActive.CompareAndSwap(false, true) {
		// wait
	}
	v = (*r32[T])(unsafe.Pointer(r)).Load()
	dmaActive.Store(false)
	return
}

func (r *R32Safe[_]) Addr() uintptr {
	return uintptr(unsafe.Pointer(r))
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

// WriteIO copies slice p to PI bus address addr using PI bus IO.  Note that all
// writes are 4-byte long and unaligned writes will write garbage to the
// remaining bytes.
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
		if shift != 0 { // first read
			mask &= 0xffff_ffff >> ((-shift) << 3)
		}
		if end-uintptr(busPtr) == 4 && endshift != 0 { // last read
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
