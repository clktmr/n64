package periph

import (
	"unsafe"

	"github.com/clktmr/n64/debug"
	"github.com/clktmr/n64/rcp/cpu"
)

// TODO protect access to DMA with mutex

// Loads bytes from PI bus into RDRAM via DMA
func DMALoad(piAddr uintptr, p []byte) (n int, err error) {
	addr := uintptr(unsafe.Pointer(unsafe.SliceData(p)))
	size := len(p)

	debug.Assert(piAddr%2 == 0, "PI start address unaligned")
	debug.Assert(size%2 == 0, "PI end address unaligned")
	debug.Assert(cpu.IsPadded(p), "Unpadded destination slice")
	debug.Assert(addr%8 == 0, "RDRAM address unaligned")

	regs.dramAddr.Store(cpu.PhysicalAddress(addr))
	regs.cartAddr.Store(cpu.PhysicalAddress(piAddr))

	cpu.InvalidateSlice(p)

	regs.writeLen.Store(uint32(size - 1))

	waitDMA()

	return size, err
}

// Stores bytes from RDRAM to PI bus via DMA
func DMAStore(piAddr uintptr, p []byte) {
	debug.Assert(piAddr%2 == 0, "PI start address unaligned")
	debug.Assert(len(p)%2 == 0, "PI end address unaligned")

	p = cpu.PaddedSlice(p)

	addr := uintptr(unsafe.Pointer(unsafe.SliceData(p)))
	debug.Assert(addr%8 == 0, "RDRAM address unaligned")
	regs.dramAddr.Store(cpu.PhysicalAddress(addr))
	regs.cartAddr.Store(cpu.PhysicalAddress(piAddr))

	cpu.WritebackSlice(p)

	regs.readLen.Store(uint32(len(p) - 1))

	waitDMA()
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
