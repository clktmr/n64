// The diplay processor is a hardware rasterizer.  It controls the texture cache
// and draws primitives directly into a framebuffer in RDRAM.  It's usually not
// used directly but through the RSP instead.
package rdp

import (
	"embedded/mmio"
	"embedded/rtos"
	"unsafe"

	"github.com/clktmr/n64/rcp"
	"github.com/clktmr/n64/rcp/cpu"
)

var regs *registers = (*registers)(unsafe.Pointer(baseAddr))

const baseAddr uintptr = cpu.KSEG1 | 0x0410_0000

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
	start mmio.R32[cpu.Addr] // Physical start address of DMA transfer
	end   mmio.R32[cpu.Addr] // Physical end address of DMA transfer

	current mmio.U32 // DMA transfer progress.  Address between start and end.  Read-only.
	status  mmio.R32[statusFlags]
	clock   mmio.U32 // 24-bit counter running at RCP frequency

	cmdBusy  mmio.U32
	pipeBusy mmio.U32
	tmemBusy mmio.U32

	// TODO there are more undocumented registers (DPC_* and DPS_*)
}

var FullSync rtos.Note

func init() {
	rcp.SetHandler(rcp.IntrRDP, handler)
	rcp.EnableInterrupts(rcp.IntrRDP)
}

//go:nosplit
//go:nowritebarrierrec
func handler() {
	rcp.ClearDPIntr()
	FullSync.Wakeup()
}

func Busy() (cmd, pipe, tmem uint32) {
	cmd = regs.cmdBusy.Load()
	pipe = regs.pipeBusy.Load()
	tmem = regs.tmemBusy.Load()
	regs.status.Store(clrBufferBusy | clrPipeBusy | clrTMEMBusy)
	return
}
