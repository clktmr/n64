package rcp

import (
	"embedded/rtos"
	"sync"
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

var notes = sync.Pool{New: func() any { return &rtos.Note{} }}

// IntrQueue queues any value safely into an interrupt context.  Multiple writer
// goroutines and a single reader are allowed.  The reader must not be
// preemptible by the writers, i.e. an interrupt.
type IntrQueue[T any] struct {
	ring              [qsize]T
	popped            [qsize]*rtos.Note
	start, end, write atomic.Int32
}

// Push returns a cleared note that gets woken up when the item is popped by the
// interrupt handler.  The borrowed note must be returned via IntrQueue.Free().
func (p *IntrQueue[T]) Push(v T) (vp *T, popped *rtos.Note) {
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
	vp = &p.ring[end]
	p.popped[end] = notes.Get().(*rtos.Note)
	popped = p.popped[end]
	popped.Clear()

	if !p.end.CompareAndSwap(end, next) {
		panic("intr queue corrupted")
	}
	return
}

func (p *IntrQueue[_]) Free(note *rtos.Note) {
	if note != nil {
		note.Wakeup()
		note.Sleep(0) // TODO takes ~800us, find better way
		notes.Put(note)
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

	p.popped[start].Wakeup()

	return
}
