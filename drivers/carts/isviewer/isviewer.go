package isviewer

import (
	"io"
	"unsafe"

	"github.com/clktmr/n64/rcp/cpu"
	"github.com/clktmr/n64/rcp/periph"
)

var regs *registers = (*registers)(unsafe.Pointer(baseAddr))

const token = 0x49533634
const baseAddr uintptr = cpu.KSEG1 | 0x13ff_0000
const bufferSize = 512 // actually 64*1024 - 0x20, but ISViewer.buf will allocate this

type registers struct {
	token    periph.U32
	readPtr  periph.U32
	_        [3]periph.U32
	writePtr periph.U32
	_        [2]periph.U32
	buf      [bufferSize / 4]periph.U32
}

type ISViewer struct {
	buf []byte
}

func Probe() *ISViewer {
	regs.token.Store(0xbeefcafe)
	if regs.token.Load() == 0xbeefcafe {
		regs.readPtr.Store(0)
		regs.writePtr.Store(0)
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

	regs.readPtr.Store(0)
	regs.writePtr.Store(uint32(n))
	regs.token.Store(token)

	for regs.readPtr.Load() != regs.writePtr.Load() {
		// wait
	}

	regs.token.Store(0x0)

	return n, err
}
