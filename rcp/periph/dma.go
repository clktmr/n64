package periph

import (
	"n64/rcp/cpu"
	"unsafe"
)

// TODO protect access to DMA with mutex

// Loads bytes from PI bus into RDRAM via DMA
func DMALoad(piAddr uintptr, size int) []byte {
	if size%2 != 0 {
		panic("unaligned dma load")
	}

	buf := cpu.MakePaddedSlice(size)
	addr := uintptr(unsafe.Pointer(unsafe.SliceData(buf)))
	regs.dramAddr.Store(cpu.PhysicalAddress(addr))
	regs.cartAddr.Store(cpu.PhysicalAddress(piAddr))

	// TODO 2 byte alignment might be needed according to n64brew wiki
	regs.writeLen.Store(uint32(size - 1))

	waitDMA()

	cpu.Invalidate(addr, len(buf))

	return buf
}

// Stores bytes from RDRAM to PI bus via DMA
func DMAStore(piAddr uintptr, p []byte) {
	buf := p

	if len(p)%2 != 0 {
		panic("unaligned dma store")
	}

	p = cpu.PaddedSlice(p)

	addr := uintptr(unsafe.Pointer(unsafe.SliceData(buf)))
	regs.dramAddr.Store(cpu.PhysicalAddress(addr))
	regs.cartAddr.Store(cpu.PhysicalAddress(piAddr))

	cpu.Writeback(addr, len(buf))

	regs.readLen.Store(uint32(len(buf) - 1))

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
