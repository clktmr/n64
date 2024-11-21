package periph

import (
	"github.com/clktmr/n64/debug"
	"github.com/clktmr/n64/rcp"
	"github.com/clktmr/n64/rcp/cpu"
)

func init() {
	rcp.SetHandler(rcp.IntrPeriph, handler)
	rcp.EnableInterrupts(rcp.IntrPeriph)
}

//go:nosplit
//go:nowritebarrierrec
func handler() {
	regs.status.Store(clearInterrupt)
}

// Loads bytes from PI bus into RDRAM via DMA
func dmaLoad(piAddr cpu.Addr, p []byte) {
	if len(p) == 0 {
		return
	}

	addr := cpu.PhysicalAddressSlice(p)
	debug.Assert(piAddr%2 == 0, "PI start address unaligned")
	debug.Assert(len(p)%2 == 0, "PI end address unaligned")
	debug.Assert(cpu.IsPadded(p), "Unpadded destination slice")
	debug.Assert(addr%8 == 0, "RDRAM address unaligned")

	waitDMA()

	regs.dramAddr.Store(addr)
	regs.cartAddr.Store(piAddr)

	cpu.InvalidateSlice(p)

	n := len(p)
	regs.writeLen.Store(uint32(n - 1))

	waitDMA()
}

// Stores bytes from RDRAM to PI bus via DMA
func dmaStore(piAddr cpu.Addr, p []byte) {
	if len(p) == 0 {
		return
	}

	addr := cpu.PhysicalAddressSlice(p)
	debug.Assert(piAddr%2 == 0, "PI start address unaligned")
	debug.Assert(len(p)%2 == 0, "PI end address unaligned")
	debug.Assert(cpu.IsPadded(p), "Unpadded destination slice")
	debug.Assert(addr%8 == 0, "RDRAM address unaligned")

	waitDMA()

	regs.dramAddr.Store(addr)
	regs.cartAddr.Store(piAddr)

	cpu.WritebackSlice(p)

	n := len(p)
	regs.readLen.Store(uint32(n - 1))
}

// Blocks until DMA has finished.
func waitDMA() {
	for {
		// TODO runtime.Gosched() ?
		if regs.status.Load()&(dmaBusy|ioBusy) == 0 {
			break
		}
	}

}
