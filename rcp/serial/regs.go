package serial

import (
	"embedded/mmio"
	"unsafe"
)

var regs *registers = (*registers)(unsafe.Pointer(BASE_ADDR))

const BASE_ADDR = uintptr(0xffffffffa480_0000)

type statusFlags uint32

const (
	dmaBusy statusFlags = 1 << iota
	ioBusy
)

type registers struct {
	dramAddr     mmio.U32
	pifReadAddr  mmio.U32
	_            mmio.U32
	_            mmio.U32
	pifWriteAddr mmio.U32
	_            mmio.U32
	status       mmio.R32[statusFlags]
}
