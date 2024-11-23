// MMIO on the PI external bus has additional sync and aligment requirements.
// Further reading: https://n64brew.dev/wiki/Memory_map#Physical_Memory_Map_accesses
package periph

import "embedded/mmio"

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
