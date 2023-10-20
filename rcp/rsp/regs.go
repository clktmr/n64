// The signal processor provides fast vector instructions.  It's usually used
// for vertex transformations and audio mixing.  It can directly control the RDP
// via XBUS or shared memory in RDRAM.  There are several precompiled microcodes
// which can be loaded to provide different functionalities.
package rsp

import (
	"embedded/mmio"
	"n64/rcp/cpu"
	"unsafe"
)

var regs *registers = (*registers)(unsafe.Pointer(baseAddr))

const baseAddr = uintptr(cpu.KSEG1 | 0x0440_0000)

type statusFlags uint32

const (
	halted statusFlags = 1 << iota
	broke
	dmaBusy
	dmaFull
	ioBusy
	singleStep
	intrOnBreak
	sig0
	sig1
	sig2
	sig3
	sig4
	sig5
	sig6
	sig7
)

type registers struct {
	addr      mmio.U32
	dramAddr  mmio.U32
	readLen   mmio.U32
	writeLen  mmio.U32
	status    mmio.R32[statusFlags]
	dmaFull   mmio.U32
	dmaBusy   mmio.U32
	semaphore mmio.U32
}
