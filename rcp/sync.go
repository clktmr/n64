package rcp

import (
	"embedded/rtos"
	"sync"
	"sync/atomic"
	"time"
)

const flagUpdated = 1 << 31

// IntrInput passes any value safely into an interrupt context using a double
// buffer.  Only a single writer and a single reader are allowed.
type IntrInput[T any] struct {
	bufs [2]T
	seq  atomic.Uint32 // bit 0: read index, bit 31: update flag
}

// Read can be used by the writer goroutine to read back the currently stored
// value.
func (p *IntrInput[T]) Read() (v T) {
	return p.bufs[p.seq.Load()&0x1]
}

// Put updates the stored value atomically.
func (p *IntrInput[T]) Put(v T) {
	new := (p.seq.Load() + 1) | flagUpdated
	p.bufs[new&0x1] = v
	p.seq.Store(new)
}

// Get returns the currently stored value and if it was updated by Put since the
// last call to Get.
//
//go:nosplit
func (p *IntrInput[T]) Get() (v T, updated bool) {
	for {
		old := p.seq.Load()
		v = p.bufs[old&0x1]
		updated = old&flagUpdated != 0

		new := old &^ flagUpdated
		if p.seq.CompareAndSwap(old, new) {
			return
		}
	}
}

const qsize = 32

// IntrQueue queues any value safely into an interrupt context.  Multiple writer
// goroutines and a single reader are allowed.  The reader must not be
// preemptible by the writers, i.e. an interrupt.
type IntrQueue[T any] struct {
	ring       [qsize]T
	start, end atomic.Int32
	mtx        sync.Mutex
	pop        rtos.Cond
}

func (p *IntrQueue[T]) Push(v T) {
	p.mtx.Lock()
	defer p.mtx.Unlock()

retry:
	p.pop.Wait(0)

	start := p.start.Load()
	end := p.end.Load()
	next := (end + 1) % int32(len(p.ring))

	if next == start {
		if !p.pop.Wait(1 * time.Second) {
			panic("dma queue timeout")
		}
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

	p.pop.Signal()

	return
}
