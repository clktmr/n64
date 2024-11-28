package periph

import (
	"embedded/rtos"
	"sync/atomic"
	"time"

	"github.com/clktmr/n64/rcp"
	"github.com/clktmr/n64/rcp/cpu"
)

type dmaDirection bool

const (
	dmaStore dmaDirection = true  // RDRAM -> PI bus
	dmaLoad  dmaDirection = false // PI bus -> RDRAM
)

type dmaJob struct {
	cart cpu.Addr
	buf  []byte
	dir  dmaDirection
	done *rtos.Note
}

func (job *dmaJob) initiate() bool {
	if job.buf == nil {
		return false
	}
	head, tail := job.split(job.buf, job.cart)
	dmaBuf, headBuf, tailBuf := job.buf[head:tail], job.buf[:head], job.buf[tail:]

	n := uint32(len(dmaBuf) - 1)
	if job.dir == dmaStore {
		WriteIO(job.cart, headBuf)
		WriteIO(job.cart+cpu.Addr(tail), tailBuf)
		if head == tail {
			return false
		}
		regs.dramAddr.Store(cpu.PhysicalAddressSlice(dmaBuf))
		regs.cartAddr.Store(job.cart + cpu.Addr(head))
		cpu.WritebackSlice(dmaBuf)
		regs.readLen.Store(n)
	} else { // dmaLoad
		if head == tail {
			return false
		}
		regs.dramAddr.Store(cpu.PhysicalAddressSlice(dmaBuf))
		regs.cartAddr.Store(job.cart + cpu.Addr(head))
		cpu.InvalidateSlice(dmaBuf)
		regs.writeLen.Store(n)
	}

	rcp.EnableInterrupts(rcp.IntrPeriph)
	return true
}

func (job *dmaJob) finish() {
	rcp.DisableInterrupts(rcp.IntrPeriph)
	if job.buf != nil {
		head, tail := job.split(job.buf, job.cart)
		if job.dir == dmaLoad {
			// Do the IO after the DMA because it might invalidate parts of
			// head and tail.
			ReadIO(job.cart, job.buf[:head])
			ReadIO(job.cart+cpu.Addr(tail), job.buf[tail:])
		}
	}

	if job.done != nil {
		job.done.Wakeup()
	}
}

func (job *dmaJob) split(p []byte, addr cpu.Addr) (head, tail int) {
	head, tail = cpu.Pads(p)

	// If DMA end address isn't 2 byte aligned, fallback to mmio for the
	// last byte.
	if (tail-head)&0x1 != 0 {
		tail -= 1
	}

	// If DMA start address isn't 2 byte aligned there is no way to use DMA
	// at all, fallback to io for the whole transfer.
	if (addr+cpu.Addr(head))&0x1 != 0 {
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
	regs.status.Store(clearInterrupt)
	rcp.SetHandler(rcp.IntrPeriph, handler)
}

//go:nosplit
//go:nowritebarrierrec
func handler() {
	regs.status.Store(clearInterrupt)
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

// dma enqueues a DMA transfer for async execution by the hardware. Returns a
// note that signals the completion of this and all previous transfers.  A nil
// note is returned if the transfer was done synchronously.
func dma(v dmaJob) (jobId uint64) {
	jobId = dmaQueue.Push(v)
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

func flush(id uint64) {
	if dmaQueue.Popped(id) {
		return
	}
	var note rtos.Note // TODO escapes
	dma(dmaJob{done: &note})
	if !note.Sleep(1 * time.Second) {
		panic("dma timeout")
	}
}
