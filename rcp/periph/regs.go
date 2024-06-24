package periph

import (
	"embedded/mmio"
	"unsafe"

	"github.com/clktmr/n64/rcp/cpu"
)

var regs *registers = (*registers)(unsafe.Pointer(baseAddr))

const baseAddr uintptr = cpu.KSEG1 | 0x0460_0000

type statusFlags uint32

// Read access to status register
const (
	dmaBusy statusFlags = 1 << iota
	ioBusy
	dmaError
	dmaFinished
)

// Write access to status register
const (
	reset statusFlags = 1 << iota
	clearInterrupt
)

type registers struct {
	dramAddr mmio.U32
	cartAddr mmio.U32
	readLen  mmio.U32
	writeLen mmio.U32
	status   mmio.R32[statusFlags]

	latch1      mmio.U32
	pulseWidth1 mmio.U32
	pageSize1   mmio.U32
	release1    mmio.U32
	latch2      mmio.U32
	pulseWidth2 mmio.U32
	pageSize2   mmio.U32
	release2    mmio.U32
}
