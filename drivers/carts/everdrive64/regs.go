// Package everdrive64 implements support for EverDrive64.
//
// Tested on EverDrive64 X7, but should also work on X3.
package everdrive64

import (
	"github.com/clktmr/n64/rcp/cpu"
	"github.com/clktmr/n64/rcp/periph"
)

type usbMode uint32

const (
	readNop  usbMode = 0xC400
	read     usbMode = 0xC600
	writeNop usbMode = 0xC000
	write    usbMode = 0xC200
)

type usbStatus uint32

const (
	act   usbStatus = 0x0200
	rxf   usbStatus = 0x0400
	txe   usbStatus = 0x0800
	power usbStatus = 0x1000
	busy  usbStatus = 0x2000
)

type registers struct {
	usbCfgR *periph.R32[usbStatus]
	usbCfgW *periph.R32[usbMode]
	version *periph.U32
	sysCfg  *periph.U32
	key     *periph.U32
}

func regs() *registers {
	return &registers{
		cpu.MMIO[periph.R32[usbStatus]](0x1f80_0004),
		cpu.MMIO[periph.R32[usbMode]](0x1f80_0004),
		cpu.MMIO[periph.U32](0x1f80_0014),
		cpu.MMIO[periph.U32](0x1f80_8000),
		cpu.MMIO[periph.U32](0x1f80_8004),
	}
}
