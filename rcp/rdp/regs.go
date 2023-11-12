package rdp

import (
	"embedded/mmio"
	"unsafe"
)

var regs *registers = (*registers)(unsafe.Pointer(BASE_ADDR))

const BASE_ADDR = uintptr(0xffffffffa410_0000)

type statusFlags uint32

// Read access to status register
const (
	xbus   statusFlags = 1 << iota // Unset to use XBUS as source for DMA transfers instead of DMEM
	freeze                         // Set to stop processing primitives
	flush                          // Set to abort all current RDP transfers immediately
	startGclk
	tmemBusy
	pipeBusy
	busy // Set from DMA transfer start until SYNC_FULL
	ready
	dmaBusy
	endPending   // Set when end register was written and transfer hasn't started yet
	startPending // Set when start register was written and transfer hasn't started yet
)

// Write access to status register
const (
	clrXbus statusFlags = 1 << iota
	setXbus
	clrFreeze
	setFreeze
	clrFlush
	setFlush
	clrTMEMBusy
	clrPipeBusy
	clrBufferBusy
	clrClock // Reset the clock register to zero
)

type registers struct {
	start   mmio.U32 // Physical start address of DMA transfer
	end     mmio.U32 // Physical end address of DMA transfer
	current mmio.U32 // DMA transfer progress.  Address between start and end.  Read-only.
	status  mmio.R32[statusFlags]
	clock   mmio.U32 // 24-bit counter running at RCP frequency

	// TODO there are more undocumented registers (DPC_* and DPS_*)
}
