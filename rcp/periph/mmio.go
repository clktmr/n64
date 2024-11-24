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

type U32 struct {
	R32[uint32]
}

type R32[T mmio.T32] struct {
	r mmio.R32[T]
}

//go:nosplit
func (r *R32[T]) Store(v T) {
	waitDMA()
	r.r.Store(v)
}

//go:nosplit
func (r *R32[T]) Load() T {
	waitDMA()
	return r.r.Load()
}

//go:nosplit
func (r *R32[T]) StoreBits(mask T, bits T) {
	waitDMA()
	r.r.StoreBits(mask, bits)
}

//go:nosplit
func (r *R32[T]) LoadBits(mask T) T {
	waitDMA()
	return r.r.LoadBits(mask)
}

//go:nosplit
func (r *R32[T]) Addr() uintptr {
	return r.r.Addr()
}

// Blocks until DMA and IO is not busy.
// TODO Looks racy.  Understand this better.  Why is it necessary?
//
//go:nosplit
func waitDMA() {
	for regs.status.Load()&(dmaBusy|ioBusy) != 0 {
		// wait
	}
}

// writeIO copies slice p to PI bus address addr using PI bus IO.  Note that all
// writes are 4-byte long and unaligned writes will write garbage to the
// remaining bytes.
func WriteIO(busAddr cpu.Addr, p []byte) {
	busPtr := unsafe.Pointer(cpu.KSEG1 | uintptr(busAddr&^0x3))
	end := uintptr(unsafe.Add(busPtr, (len(p)+3)&^0x3))
	shift := -(int(busAddr) & 0x3)

	pPtr := unsafe.Pointer(unsafe.SliceData(p))
	pPtr = unsafe.Add(pPtr, shift)
	for uintptr(busPtr) <= end {
		var data uint32
		if uintptr(pPtr)&0x3 == 0 {
			data = *(*uint32)(pPtr)
		} else { // unaligned access forbidden on mips
			p := *(*[4]byte)(pPtr)
			data = uint32(p[0])<<24 | uint32(p[1])<<16 | uint32(p[2])<<8 | uint32(p[3])
		}
		(*U32)(busPtr).Store(data)

		pPtr = unsafe.Add(pPtr, 4)
		busPtr = unsafe.Add(busPtr, 4)
	}
}

// readIO copies from PI bus address addr to slice p using PI bus IO.
func ReadIO(busAddr cpu.Addr, p []byte) {
	busPtr := unsafe.Pointer(cpu.KSEG1 | uintptr(busAddr&^0x3))
	end := uintptr(unsafe.Add(busPtr, (len(p)+3)&^0x3))
	shift := -(int(busAddr) & 0x3)

	pPtr := unsafe.Pointer(unsafe.SliceData(p))
	pPtr = unsafe.Add(pPtr, shift)
	for uintptr(busPtr) <= end {
		data := (*U32)(busPtr).Load()
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
