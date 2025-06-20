// Package isviewer provides a logging via an ISViewer devices.
//
// ISViewer was a development cartridge with logging capabilities. While the
// original hardware is not available anymore, it can be emulated by ares
// emulator as well as SummerCart64. To enable ISViewer emulation on
// SummerCart64 use the sc64deployer utility:
//
//	sc64deployer debug --isv 0x03FF0000
package isviewer

import (
	"github.com/clktmr/n64/rcp/cpu"
	"github.com/clktmr/n64/rcp/periph"
)

func regs() *registers { return cpu.MMIO[registers](0x13ff_0000) }

const token = 0x49533634
const bufferSize = 0x1400_0000 - 0x13ff_0020

type registers struct {
	token    periph.U32
	readPtr  periph.U32
	_        [3]periph.U32
	writePtr periph.U32
	_        [2]periph.U32
	buf      [bufferSize / 4]periph.U32
}

var piBuf = periph.NewDevice(cpu.PhysicalAddressSlice(regs().buf[:]), bufferSize)

type Cart struct{}

// Probe reports the ISV
func Probe() *Cart {
	regs().token.Store(0xbeefcafe)
	if regs().token.Load() == 0xbeefcafe {
		regs().readPtr.Store(0)
		regs().writePtr.Store(0)
		return &Cart{}
	}
	return nil
}

func (v *Cart) Write(p []byte) (n int, err error) {
	for len(p) > 0 {
		var nn int
		nn, err = piBuf.WriteAt(p[:min(len(p), piBuf.Size())], 0)
		if err != nil {
			return
		}
		p = p[nn:]

		regs().readPtr.Store(0)
		regs().writePtr.Store(uint32(nn))
		regs().token.Store(token)

		for regs().readPtr.Load() != regs().writePtr.Load() {
			// wait
		}

		regs().token.Store(0x0)
		n += nn
	}

	return
}
