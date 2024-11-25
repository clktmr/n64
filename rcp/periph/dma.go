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
	cart cpu.Addr
	buf  []byte
	dir  dmaDirection
}

func (job *dmaJob) initiate() bool {
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
			ReadIO(job.cart, headBuf)
			ReadIO(job.cart+cpu.Addr(tail), tailBuf)
			return false
		}
		regs.dramAddr.Store(cpu.PhysicalAddressSlice(dmaBuf))
		regs.cartAddr.Store(job.cart + cpu.Addr(head))
		cpu.InvalidateSlice(dmaBuf)
		regs.writeLen.Store(n)
	}
	return true
}

func (job *dmaJob) finish() {
	head, tail := job.split(job.buf, job.cart)
	if job.dir == dmaLoad {
		// Do the IO after the DMA because it might invalidate parts of
		// head and tail.
		ReadIO(job.cart, job.buf[:head])
		ReadIO(job.cart+cpu.Addr(tail), job.buf[tail:])
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
var dmaActive atomic.Bool

func init() {
	regs.status.Store(clearInterrupt)
	rcp.SetHandler(rcp.IntrPeriph, handler)
	rcp.EnableInterrupts(rcp.IntrPeriph)
}

//go:nosplit
//go:nowritebarrierrec
func handler() {
	regs.status.Store(clearInterrupt)

	job, ok := dmaQueue.Pop()
	if !ok {
		panic("unexpected dma intr")
	}
	job.finish()

next:
	job, ok = dmaQueue.Peek()
	if !ok {
		wasActive := dmaActive.Swap(false)
		if !wasActive {
			panic("broken dma sync")
		}
		return
	}

	if !job.initiate() {
		_, _ = dmaQueue.Pop()
		goto next
	}
}

// enqueueTransfer enqueues a DMA transfer for execution by the hardware.
// Returns a note that signals the completion of this and all previous
// transfers.
func dma(piAddr cpu.Addr, p []byte, dir dmaDirection) (done *rtos.Note) {
	job := dmaJob{piAddr, p, dir}
	done = dmaQueue.Push(job)
	if !dmaActive.Swap(true) {
		// initially trigger dma queue
		if activated := job.initiate(); !activated {
			_, _ = dmaQueue.Pop()
			dmaActive.Store(false)
		}
	}

	return
}
