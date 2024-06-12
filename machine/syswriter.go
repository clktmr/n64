// Package machine is imported by the runtime and allows the target to implement
// some hooks, most importantly rt0.
package machine

import (
	"unsafe"

	"github.com/clktmr/n64/rcp/cpu"
	"github.com/clktmr/n64/rcp/periph"
)

// TODO use register definitions from isviewer package
var regs *registers = (*registers)(unsafe.Pointer(baseAddr))

const baseAddr = uintptr(cpu.KSEG1 | 0x13ff_0014)
const bufferSize = 512

type registers struct {
	writeLen periph.U32
	_        periph.U32
	_        periph.U32
	buf      [128]periph.U32
}

// Writes to ISViewer registers, regardless if a ISViewer is present or not.  Is
// rather slow, because it avoids using DMA.  Only intended as a fail safe
// logger in very early boot.
//
//go:nowritebarrierrec
//go:nosplit
func DefaultWrite(fd int, p []byte) int {
	n := len(p)
	if n > bufferSize {
		n = bufferSize
	}

	for i := 0; i < n/4; i++ {
		pi := 4 * i
		regs.buf[i].Store(0 |
			uint32(p[pi])<<24 |
			uint32(p[pi+1])<<16 |
			uint32(p[pi+2])<<8 |
			uint32(p[pi+3]))
	}

	if n%4 != 0 {
		var tail uint32
		for i := 0; i < n%4; i++ {
			base := len(p) - n%4
			tail |= uint32(p[base+i]) << ((3 - i) * 8)
		}
		regs.buf[n/4].Store(tail)
	}

	regs.writeLen.Store(uint32(n))

	return n
}
