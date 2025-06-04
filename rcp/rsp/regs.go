// Package rsp provides loading and running microcode on the signal processor.
//
// The signal processor provides fast vector instructions. It's usually used for
// vertex transformations and audio mixing. It can directly control the RDP via
// XBUS or shared memory in RDRAM. There are several precompiled microcodes
// which can be loaded to provide different functionalities.
package rsp

import (
	"embedded/mmio"
	"unsafe"

	"github.com/clktmr/n64/rcp/cpu"
)

// RSP program counter. Access only allowed when RSP is halted.
var pc *mmio.U32 = (*mmio.U32)(unsafe.Pointer(cpu.KSEG1 | 0x0408_0000))

var regs *registers = (*registers)(unsafe.Pointer(baseAddr))

const baseAddr uintptr = cpu.KSEG1 | 0x0404_0000

type statusFlags uint32

// Read access to status register
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

// Write access to status register
const (
	clrHalt statusFlags = 1 << iota
	setHalt
	clrBroke
	clrIntr
	setIntr
	clrSingleStep
	setSingleStep
	clrIntbreak
	setIntbreak
	clrSig0
	setSig0
	clrSig1
	setSig1
	clrSig2
	setSig2
	clrSig3
	setSig3
	clrSig4
	setSig4
	clrSig5
	setSig5
	clrSig6
	setSig6
	clrSig7
	setSig7
)

type registers struct {
	rspAddr   mmio.R32[cpu.Addr]
	rdramAddr mmio.R32[cpu.Addr]
	readLen   mmio.U32
	writeLen  mmio.U32
	status    mmio.R32[statusFlags]
	dmaFull   mmio.U32
	dmaBusy   mmio.U32
	semaphore mmio.U32
}

const (
	DMEM = Memory(0x0400_0000)
	IMEM = Memory(0x0400_1000)
)
