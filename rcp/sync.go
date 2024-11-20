package rcp

import (
	"sync/atomic"
)

// IntrInput passes any value safely into an interrupt context.  Only a single
// writer goroutine and a single reader are allowed.  The reader must not be
// preemptible by the writer, i.e. an interrupt.
type IntrInput[T any] struct {
	next    int32 // owned by writer
	current int32 // owned by reader

	bufs [2]T
	ptr  atomic.Int32
}

// Get can be used by the writer goroutine to read back the currently stored
// value.
func (p *IntrInput[T]) Get() (v T) {
	return p.bufs[(p.next+1)&0x1]
}

func (p *IntrInput[T]) Store(v T) {
	// Write alternating to bufs[0] and bufs[1] and set ptr to the latest
	// write.  Will never write where ptr points at.
	p.bufs[p.next] = v
	p.ptr.Store(p.next)
	p.next = (p.next + 1) & 0x1

	// Write zero value in the unused buffer to avoid holding hidden
	// references that might prevent freeing memory.
	var zero T
	p.bufs[p.next] = zero
}

//go:nosplit
func (p *IntrInput[T]) Load() (v T, updated bool) {
	ptr := p.ptr.Swap(-1)
	// Since we aren't preemptible by the writer, we can read *ptr safely.
	if ptr == -1 {
		return p.bufs[p.current], false
	}
	p.current = ptr
	return p.bufs[p.current], true
}
