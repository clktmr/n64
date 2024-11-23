package periph

import (
	"embedded/rtos"
	"sync/atomic"

	"github.com/clktmr/n64/debug"
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

func (job *dmaJob) initiate() {
	regs.dramAddr.Store(cpu.PhysicalAddressSlice(job.buf))
	regs.cartAddr.Store(job.cart)
	n := uint32(len(job.buf) - 1)
	if job.dir == dmaStore {
		cpu.WritebackSlice(job.buf)
		regs.readLen.Store(n)
	} else { // dmaLoad
		cpu.InvalidateSlice(job.buf)
		regs.writeLen.Store(n)
	}
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

	_, ok := dmaQueue.Pop()
	if !ok {
		panic("unexpected dma intr")
	}

	job, ok := dmaQueue.Peek()
	if !ok {
		wasActive := dmaActive.Swap(false)
		if !wasActive {
			panic("broken dma sync")
		}
		return
	}

	job.initiate()
}

// enqueueTransfer enqueues a DMA transfer for execution by the hardware.
// Returns a note that signals the completion of this and all previous
// transfers.
func dma(piAddr cpu.Addr, p []byte, dir dmaDirection) (done *rtos.Note) {
	addr := cpu.PhysicalAddressSlice(p)
	debug.Assert(piAddr%2 == 0, "PI start address unaligned")
	debug.Assert(len(p)%2 == 0, "PI end address unaligned")
	debug.Assert(len(p) != 0, "PI dma empty slice")
	debug.Assert(cpu.IsPadded(p), "Unpadded destination slice")
	debug.Assert(addr%8 == 0, "RDRAM address unaligned")

	job := dmaJob{piAddr, p, dir}
	done = dmaQueue.Push(job)
	if !dmaActive.Swap(true) {
		job.initiate() // initially trigger dma queue
	}

	return
}
