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

const qsize = 32

// IntrQueue queues any value safely into an interrupt context.  Multiple writer
// goroutines and a single reader are allowed.  The reader must not be
// preemptible by the writers, i.e. an interrupt.
type IntrQueue[T any] struct {
	ring              [qsize]T
	start, end, write atomic.Int32
}

func (p *IntrQueue[T]) Push(v T) {
retry:
	start := p.start.Load()
	end := p.end.Load()
	next := (end + 1) % int32(len(p.ring))
	if next == start {
		goto retry
	}

	if !p.write.CompareAndSwap(end, next) {
		goto retry
	}

	p.ring[end] = v

	if !p.end.CompareAndSwap(end, next) {
		panic("intr queue corrupted")
	}
}

//go:nosplit
func (p *IntrQueue[T]) Peek() (v *T, ok bool) {
	start := p.start.Load()
	end := p.end.Load()
	if end == start {
		return v, false
	}

	return &p.ring[start], true
}

//go:nosplit
func (p *IntrQueue[T]) Pop() (v *T, ok bool) {
	start := p.start.Load()
	end := p.end.Load()
	if end == start {
		return v, false
	}

	v = &p.ring[start]
	ok = true

	// Write zero value in the unused buffer to avoid holding hidden
	// references that might prevent freeing memory.
	// TODO not possible due to go:nowritebarrierrec
	// var zero T
	// p.ring[start] = zero

	if !p.start.CompareAndSwap(start, (start+1)%int32(len(p.ring))) {
		panic("multiple readers")
	}

	return
}
