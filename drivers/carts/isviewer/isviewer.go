package isviewer

import (
	"unsafe"

	"github.com/clktmr/n64/rcp/cpu"
	"github.com/clktmr/n64/rcp/periph"
)

var regs *registers = (*registers)(unsafe.Pointer(baseAddr))

const token = 0x49533634
const baseAddr uintptr = cpu.KSEG1 | 0x13ff_0000
const bufferSize = 0x1400_0000 - 0x13ff_0020

type registers struct {
	token    periph.U32
	readPtr  periph.U32
	_        [3]periph.U32
	writePtr periph.U32
	_        [2]periph.U32
	buf      [bufferSize / 4]periph.U32
}

var piBuf *periph.Device = periph.NewDevice(cpu.PhysicalAddress(regs.buf[0].Addr()), bufferSize)

type Cart struct{}

func Probe() *Cart {
	regs.token.Store(0xbeefcafe)
	if regs.token.Load() == 0xbeefcafe {
		regs.readPtr.Store(0)
		regs.writePtr.Store(0)
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

		regs.readPtr.Store(0)
		regs.writePtr.Store(uint32(nn))
		regs.token.Store(token)

		for regs.readPtr.Load() != regs.writePtr.Load() {
			// wait
		}

		regs.token.Store(0x0)
		n += nn
	}

	return
}
