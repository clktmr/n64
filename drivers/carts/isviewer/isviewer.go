package isviewer

import (
	"io"
	"unsafe"

	"github.com/clktmr/n64/rcp/cpu"
	"github.com/clktmr/n64/rcp/periph"
)

var regs *registers = (*registers)(unsafe.Pointer(baseAddr))

const token = 0x49533634
const baseAddr = uintptr(cpu.KSEG1 | 0x13ff_0000)
const bufferSize = 64*1024 - 0x20

type registers struct {
	token    periph.U32
	readPtr  periph.U32
	_        periph.U32
	_        periph.U32
	_        periph.U32
	writePtr periph.U32
	_        periph.U32
	_        periph.U32
	buf      [bufferSize / 4]periph.U32
}

type ISViewer struct {
	buf []byte
}

func Probe() *ISViewer {
	regs.token.Store(token)
	if regs.token.Load() == token {
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

	wp := (regs.writePtr.Load() + uint32(n)) % bufferSize
	regs.writePtr.Store(wp)

	for regs.readPtr.Load() != regs.writePtr.Load() {
		// wait
	}

	return n, err
}
