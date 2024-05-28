package rsp

import (
	"n64/rcp/cpu"
	"runtime"
	"unsafe"
)

// TODO protect access to DMA with mutex

// Loads bytes from RSP IMEM/DMEM into RDRAM via DMA
func DMALoad(rspAddr uintptr, size int, bank memoryBank) []byte {
	if rspAddr%8 != 0 {
		panic("rsp: unaligned dma load")
	}

	buf := cpu.MakePaddedSlice(size)
	addr := uintptr(unsafe.Pointer(unsafe.SliceData(buf)))
	regs.rdramAddr.Store(uint32(addr))
	regs.rspAddr.Store(uint32(uintptr(bank) + rspAddr))

	regs.writeLen.Store(uint32(size - 1))

	waitDMA()

	cpu.Invalidate(addr, len(buf))

	return buf
}

// Stores bytes from RDRAM to RSP IMEM/DMEM via DMA
func DMAStore(rspAddr uintptr, p []byte, bank memoryBank) {
	if rspAddr%8 != 0 {
		panic("rsp: unaligned dma store")
	}

	buf := p

	if cpu.IsPadded(p) == false {
		buf = cpu.MakePaddedSlice(len(p))
		copy(buf, p)
	}

	addr := uintptr(unsafe.Pointer(unsafe.SliceData(buf)))
	regs.rdramAddr.Store(uint32(addr))
	regs.rspAddr.Store(uint32(uintptr(bank) + rspAddr))

	cpu.Writeback(addr, len(buf))

	regs.readLen.Store(uint32(len(buf) - 1))

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
