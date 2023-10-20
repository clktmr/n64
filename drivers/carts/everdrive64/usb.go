package everdrive64

import (
	"io"
	"n64/rcp/cpu"
	"n64/rcp/periph"
)

const bufferSize = 512

type EverDrive64 struct {
	buf []byte
}

func Probe() *EverDrive64 {
	regs.key.Store(0xaa55) // magic key to unlock registers
	switch regs.version.Load() {
	case 0xed64_0008: // EverDrive64 X3
		fallthrough
	case 0xed64_0013: // EverDrive64 X7
		cart := &EverDrive64{
			buf: cpu.MakePaddedSlice(bufferSize),
		}
		// TODO move implementation of UNFLoader protocol into separate package
		cart.Write([]byte{'D', 'M', 'A', '@', 5, 0, 0, 4, 0, 2, 0, 1, 'C', 'M', 'P', 'H'})
		return cart
	}
	return nil
}

func (v *EverDrive64) Write(p []byte) (n int, err error) {
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

	// EverDrive64 needs 2 byte alignment, not only for DMA
	written := n - n%2

	offset := bufferSize - written
	regs.usbCfgW.Store(writeNop)
	periph.DMAStore(regs.usbData[0].Addr()+uintptr(offset), p[:written])
	regs.usbCfgW.Store(write | usbMode(offset))

	for {
		if regs.usbCfgR.Load()&act == 0 {
			break
		}
	}

	return written, err
}

func (v *EverDrive64) SystemWriter(fd int, p []byte) int {
	// TODO move implementation of UNFLoader protocol into separate package

	s := len(p)
	if s >= 1<<24 {
		s = 1 << 24
	}
	v.Write([]byte{'D', 'M', 'A', '@', 1, byte(s >> 16), byte(s >> 8), byte(s)})

	written := 0
	for written < s-s%2 {
		n, _ := v.Write(p[written:])
		written += n
	}

	footer := []byte{p[len(p)-1], 'C', 'M', 'P', 'H', '0'}

	// Ensure 2 byte alignment
	if s%2 == 0 {
		footer = footer[1 : len(footer)-1]
	}
	v.Write(footer)

	return written
}
