package isviewer

import (
	"io"
	"unsafe"

	"github.com/clktmr/n64/rcp/cpu"
	"github.com/clktmr/n64/rcp/periph"
)

var regs *registers = (*registers)(unsafe.Pointer(baseAddr))

const baseAddr = uintptr(cpu.KSEG1 | 0x13ff_0014)
const bufferSize = 512

type registers struct {
	writeLen periph.U32
	_        periph.U32
	_        periph.U32
	buf      [128]periph.U32
}

type ISViewer struct {
	buf []byte
}

func Probe() *ISViewer {
	const probeVal = 0xbeefcafe
	regs.buf[0].Store(probeVal)
	if regs.buf[0].Load() == probeVal {
		return &ISViewer{
			buf: cpu.MakePaddedSlice[byte](bufferSize),
		}
	}
	return nil
}

func (v *ISViewer) Write(p []byte) (n int, err error) {
	n = len(p)
	if n > bufferSize {
		n = bufferSize
		err = io.ErrShortWrite
	}

	// If used as a SystemWriter we might be in a syscall.  Make sure we
	// don't allocate in DMAStore, or we might panic with "malloc during
	// signal".
	if cpu.IsPadded(p) == false {
		copy(v.buf, p)
		p = v.buf
	}

	periph.DMAStore(regs.buf[0].Addr(), p[:n+n%2])

	regs.writeLen.Store(uint32(n))

	return n, err
}
