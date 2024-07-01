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
	buf   []byte
	piBuf *periph.Device
}

func Probe() *ISViewer {
	regs.token.Store(0xbeefcafe)
	if regs.token.Load() == 0xbeefcafe {
		regs.readPtr.Store(0)
		regs.writePtr.Store(0)
		return &ISViewer{
			buf:   cpu.MakePaddedSlice[byte](bufferSize),
			piBuf: periph.NewDevice(regs.buf[0].Addr(), bufferSize),
		}
	}
	return nil
}

func (v *ISViewer) Write(p []byte) (n int, err error) {
	// If used as a SystemWriter we might be in a syscall.  Make sure we
	// don't allocate in periph/Device.Write().
	if cpu.IsPadded(p) == false {
		n = copy(v.buf, p)
		p = v.buf[:n]
	}

	v.piBuf.Seek(io.SeekStart, 0)
	n, err = v.piBuf.Write(p)
	v.piBuf.Flush()

	regs.readPtr.Store(0)
	regs.writePtr.Store(uint32(n))
	regs.token.Store(token)

	for regs.readPtr.Load() != regs.writePtr.Load() {
		// wait
	}

	regs.token.Store(0x0)

	return n, err
}
