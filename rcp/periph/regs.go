package periph

import (
	"embedded/mmio"

	"github.com/clktmr/n64/rcp/cpu"
)

func regs() *registers { return cpu.MMIO[registers](0x0460_0000) }

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
	dramAddr mmio.R32[cpu.Addr]
	cartAddr mmio.R32[cpu.Addr]
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
