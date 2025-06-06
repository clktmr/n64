// Package rsp provides loading and running microcode on the signal processor.
//
// The signal processor provides fast vector instructions. It's usually used for
// vertex transformations and audio mixing. It can directly control the RDP via
// XBUS or shared memory in RDRAM. There are several precompiled microcodes
// which can be loaded to provide different functionalities.
package rsp

import (
	"embedded/mmio"

	"github.com/clktmr/n64/rcp/cpu"
)

// RSP program counter. Access only allowed when RSP is halted.
var pc = cpu.MMIO[mmio.R32[cpu.Addr]](0x0408_0000)
var regs = cpu.MMIO[registers](0x0404_0000)

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

func SetInterrupt(en bool) {
	if en {
		regs.status.Store(setIntbreak)
	} else {
		regs.status.Store(clrIntbreak)
	}
}

func Halted() bool { return regs.status.LoadBits(halted) != 0 }
func Broke() bool  { return regs.status.LoadBits(broke) != 0 }
func Resume()      { regs.status.Store(clrBroke | clrHalt) }
func Step() {
	regs.status.Store(setSingleStep)
	Resume()
	for !Halted() {
		// wait
	}
}

func Signals() uint8       { return uint8(regs.status.Load() >> 7) }
func SetSignals(s uint8)   { regs.status.Store(statusFlags(interleave(s)) << 10) }
func ClearSignals(s uint8) { regs.status.Store(statusFlags(interleave(s)) << 9) }

func SetSignalsMask(s uint8) uint32   { return uint32(interleave(s)) << 10 }
func ClearSignalsMask(s uint8) uint32 { return uint32(interleave(s)) << 9 }

// interleave puts a zero bit before every bit in mask.
func interleave(mask uint8) (r uint16) {
	r = uint16(mask)
	r = (r ^ (r << 4)) & 0x0f0f
	r = (r ^ (r << 2)) & 0x3333
	r = (r ^ (r << 1)) & 0x5555
	return
}

// PC returns the RSP's current program counter value. Can only be read while
// halted, otherwise returns 0.
func PC() cpu.Addr {
	if Halted() {
		return pc.Load()
	}
	return 0xffff_ffff
}
