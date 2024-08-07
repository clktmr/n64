package rsp

import (
	"runtime"
	"unsafe"

	"github.com/clktmr/n64/debug"
	"github.com/clktmr/n64/rcp/cpu"
)

// TODO protect access to DMA with mutex

// Loads bytes from RSP IMEM/DMEM into RDRAM via DMA
func DMALoad(rspAddr uintptr, size int, bank memoryBank) []byte {
	debug.Assert(rspAddr%8 == 0, "rsp: unaligned dma load")

	buf := cpu.MakePaddedSlice[byte](size)
	addr := uintptr(unsafe.Pointer(unsafe.SliceData(buf)))
	regs.rdramAddr.Store(uint32(addr))
	regs.rspAddr.Store(uint32(uintptr(bank) + rspAddr))

	cpu.InvalidateSlice(buf)

	regs.writeLen.Store(uint32(size - 1))

	waitDMA()

	return buf
}

// Stores bytes from RDRAM to RSP IMEM/DMEM via DMA
func DMAStore(rspAddr uintptr, p []byte, bank memoryBank) {
	debug.Assert(rspAddr%8 == 0, "rsp: unaligned dma store")

	p = cpu.CopyPaddedSlice(p)

	addr := uintptr(unsafe.Pointer(unsafe.SliceData(p)))
	regs.rdramAddr.Store(uint32(addr))
	regs.rspAddr.Store(uint32(uintptr(bank) + rspAddr))

	cpu.WritebackSlice(p)

	regs.readLen.Store(uint32(len(p) - 1))

	waitDMA()
}

// Blocks until DMA has finished.
func waitDMA() {
	for {
		if regs.status.Load()&(dmaBusy|ioBusy) == 0 {
			break
		}
		runtime.Gosched()
	}
}
