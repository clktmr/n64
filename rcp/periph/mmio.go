// MMIO on the PI external bus has additional sync and aligment requirements.
// Further reading: https://n64brew.dev/wiki/Memory_map#Physical_Memory_Map_accesses
package periph

import "embedded/mmio"

// Use for MMIO on the PI external bus:
// - 0x0500_0000 to 0x1fbf_ffff
// - 0x1fd0_0000 to 0x7fff_ffff
type U32 struct {
	mmio.U32
}

func (r *U32) Store(v uint32) {
	r.U32.Store(v)

	for regs.status.LoadBits(ioBusy) != 0 {
		// wait
	}
}

type R32[T mmio.T32] struct {
	mmio.R32[T]
}

func (r *R32[T]) Store(v T) {
	r.R32.Store(v)

	for regs.status.LoadBits(ioBusy) != 0 {
		// wait
	}
}
