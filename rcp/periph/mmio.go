// MMIO on the PI external bus has additional sync and aligment requirements.
// Further reading: https://n64brew.dev/wiki/Memory_map#Physical_Memory_Map_accesses
package periph

import "embedded/mmio"

// Use for MMIO on the PI external bus:
// - 0x0500_0000 to 0x1fbf_ffff
// - 0x1fd0_0000 to 0x7fff_ffff
type U32 struct {
	r mmio.U32
}

func (r *U32) Store(v uint32) {
	r.r.Store(v)

	for {
		if regs.status.LoadBits(ioBusy) == 0 {
			break
		}
	}
}

func (r *U32) Load() uint32 {
	return r.r.Load()
}

func (r *U32) Addr() uintptr {
	return r.r.Addr()
}

type R32[T mmio.T32] struct {
	r mmio.R32[T]
}

func (r *R32[T]) Store(v T) {
	r.r.Store(v)

	for {
		if regs.status.LoadBits(ioBusy) == 0 {
			break
		}
	}
}

func (r *R32[T]) Load() T {
	return r.r.Load()
}

func (r *R32[T]) Addr() uintptr {
	return r.r.Addr()
}
