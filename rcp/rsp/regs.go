package rsp

import (
	"embedded/mmio"
	"unsafe"
)

var regs *registers = (*registers)(unsafe.Pointer(BASE_ADDR))

const BASE_ADDR = uintptr(0xffffffffa440_0000)

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
