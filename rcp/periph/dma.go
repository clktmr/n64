package periph

import (
	"embedded/rtos"
	"sync/atomic"

	"github.com/clktmr/n64/rcp"
	"github.com/clktmr/n64/rcp/cpu"
)

type dmaDirection bool

const (
	dmaStore dmaDirection = true  // RDRAM -> PI bus
	dmaLoad  dmaDirection = false // PI bus -> RDRAM
)

type dmaJob struct {
	buf  []byte
	cart cpu.Addr
	dir  dmaDirection
	done *rtos.Cond
}

// initiate returns true if a dma transfer was started.  If it returns false,
// that means the whole job did fallback to mmio.
func (job *dmaJob) initiate() bool {
	if job.buf == nil {
		return false
	}
	head, tail := job.split()
	dmaBuf, headBuf, tailBuf := job.buf[head:tail], job.buf[:head], job.buf[tail:]

	n := uint32(len(dmaBuf) - 1)
	if job.dir == dmaStore {
		rcp.WriteIO[*u32](job.cart, headBuf)
		rcp.WriteIO[*u32](job.cart+cpu.Addr(tail), tailBuf)
		if head == tail {
			return false
		}
		regs().dramAddr.Store(cpu.PhysicalAddressSlice(dmaBuf))
		regs().cartAddr.Store(job.cart + cpu.Addr(head))
		cpu.WritebackSlice(dmaBuf)
		regs().readLen.Store(n)
	} else { // dmaLoad
		if head == tail {
			return false
		}
		regs().dramAddr.Store(cpu.PhysicalAddressSlice(dmaBuf))
		regs().cartAddr.Store(job.cart + cpu.Addr(head))
		cpu.InvalidateSlice(dmaBuf)
		regs().writeLen.Store(n)
	}

	rcp.EnableInterrupts(rcp.IntrPeriph)
	return true
}

// finish does remaining mmio and wakeups any waiter on the job's note.
func (job *dmaJob) finish() {
	rcp.DisableInterrupts(rcp.IntrPeriph)
	if job.buf != nil {
		head, tail := job.split()
		if job.dir == dmaLoad {
			// Do the IO after the DMA because it might invalidate parts of
			// head and tail.
			rcp.ReadIO[*u32](job.cart, job.buf[:head])
			rcp.ReadIO[*u32](job.cart+cpu.Addr(tail), job.buf[tail:])
		}
	}

	if job.done != nil {
		job.done.Signal()
	}
}

// split returns two positions which split job.buf in three parts. The slice
// job.buf[head:tail] is safe for DMA, job.buf[:head] and job.buf[tail:] must
// fallback to mmio.
func (job *dmaJob) split() (head, tail int) {
	head, tail = cpu.Pads(job.buf)

	// If DMA length isn't 2 byte aligned, fallback to mmio for last byte.
	if (tail-head)&0x1 != 0 {
		tail -= 1
	}

	// If DMA start address isn't 2 byte aligned there is no way to use DMA
	// at all, fallback to mmio for the whole transfer.
	if (job.cart+cpu.Addr(head))&0x1 != 0 {
		head = 0
		tail = 0
	}

	return
}

var dmaQueue rcp.IntrQueue[dmaJob]

// If true: No PI interrupts scheduled, dmaQueue can be read.
// If false: A PI interrupt will trigger and read the dmaQueue.
var dmaActive atomic.Bool // TODO rename dmaQueueLock, make spinlock

func init() {
	regs().status.Store(clearInterrupt)
	rcp.SetHandler(rcp.IntrPeriph, handler)
}

//go:nosplit
//go:nowritebarrierrec
func handler() {
	regs().status.Store(clearInterrupt)
	dmaActive.Store(false)

	job, ok := dmaQueue.Pop()
	if !ok {
		panic("unexpected dma intr")
	}
	job.finish()

next:
	if job, ok = dmaQueue.Peek(); !ok {
		return
	}

	if !job.initiate() {
		if job, ok = dmaQueue.Pop(); !ok {
			panic("empty dma queue")
		}
		job.finish()
		goto next
	}

	dmaActive.Store(true)
}

// dma enqueues a DMA transfer for async execution by the hardware.
func dma(v dmaJob) {
	dmaQueue.Push(v)
	// might preempt here, but that's ok
	if !dmaActive.Swap(true) {
		// initially trigger dma queue
		for {
			job, ok := dmaQueue.Peek()
			if !ok {
				dmaActive.Store(false)
				return
			}
			if activated := job.initiate(); activated {
				return
			}
			job.finish()
			dmaQueue.Pop()
		}
	}

	return
}
