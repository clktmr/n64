// The serial interface provides access to the PIF microchip, which in turn
// handles console startup, reset and most importantly the joyBus.  The joyBus
// is connected to the controllers and their accessories, e.g. rumble pak.
//
// The serial interface is very slow.  Blocking reads and writes should be
// avoided.
package serial

import (
	"embedded/mmio"
	"unsafe"

	"github.com/clktmr/n64/rcp/cpu"
)

var regs *registers = (*registers)(unsafe.Pointer(baseAddr))

const baseAddr uintptr = cpu.KSEG1 | 0x0480_0000

const (
	pifRamAddr uint32 = 0x1fc0_07c0
	pifRamSize        = 64
)

type statusFlags uint32

const (
	dmaBusy statusFlags = 1 << iota
	ioBusy
)

type registers struct {
	dramAddr       mmio.U32
	pifReadAddr    mmio.U32 // Writing triggers the actual joybus exchange
	pifWriteAddr4B mmio.U32
	_              mmio.U32
	pifWriteAddr   mmio.U32
	pifReadAddr4B  mmio.U32
	status         mmio.R32[statusFlags]
}
