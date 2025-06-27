package cpu

import (
	"runtime"
	"slices"
	"unsafe"
)

// Pinner is a lightweight version of runtime.Pinner. In contrast to
// runtime.Pinner it's not compatible with cgocheck=1.
type Pinner struct {
	*pinner
}

type pinner struct {
	// The object is pinned by keeping a reference from heap to it. This
	// will also enforce it to escape, since only pointers on the stack can
	// point into the stack. This is necessary because in the current
	// runtime implementation (go1.24) the stack might be moved.
	refs []unsafe.Pointer
}

type eface struct {
	_type, data unsafe.Pointer
}

func (p *Pinner) Pin(pointer any) {
	if p.pinner == nil {
		p.pinner = new(pinner)
		p.refs = make([]unsafe.Pointer, 0, 8)
		runtime.SetFinalizer(p.pinner, func(i *pinner) {
			if len(i.refs) != 0 {
				panic("cpu.Pinner: memory leak")
			}
		})

	}
	itf := (*eface)(unsafe.Pointer(&pointer))

	// TODO debug.Assert(pointer holds pointer type)

	// In contrast to runtime.Pinner, we'll only add the pointer if it's not
	// already pinned to keep p.refs small.
	if !slices.Contains(p.refs, itf.data) {
		p.refs = append(p.refs, itf.data)
	}
}

func (p *Pinner) Unpin() {
	// In contrast to runtime.Pinner, we clear p.refs instead of dropping
	// the reference to it to avoid an allocation next time
	clear(p.refs[:])
	p.refs = p.refs[:0]
}

func PinSlice[T any](p *Pinner, slice []T) {
	p.Pin(unsafe.SliceData(slice))
}
