package rspq

import (
	"unsafe"

	"github.com/clktmr/n64/debug"
	"github.com/clktmr/n64/rcp/cpu"
)

type Command byte

const (
	CmdWaitNewInput    Command = 0x00
	CmdNoop            Command = 0x01
	CmdJump            Command = 0x02
	CmdCall            Command = 0x03
	CmdRet             Command = 0x04
	CmdDma             Command = 0x05
	CmdWriteStatus     Command = 0x06
	CmdSwapBuffers     Command = 0x07
	CmdTestWriteStatus Command = 0x08
	CmdRdpWaitIdle     Command = 0x09
	CmdRdpSetBuffer    Command = 0x0A
	CmdRdpAppendBuffer Command = 0x0B
)

func dma(rdramAddr uintptr, dmemAddr cpu.Addr, n uint32, flags uint32) {
	debug.Assert(dmemAddr&0x7 == 0 && n&0x7 == 0, "unaligned dma")
	Write(CmdDma, uint32(cpu.PhysicalAddress(rdramAddr)), uint32(dmemAddr), n-1, flags)
}

const dmaBusyOrFull = 12

// dmaWrite enqueues a DMA write command (dmem to rdram)
func DMAWrite(p []byte, addr cpu.Addr, n uint32) {
	cpu.InvalidateSlice(p)
	rdramAddr := uintptr(unsafe.Pointer(unsafe.SliceData(p)))
	dma(rdramAddr, addr, n, 0xffff_8000|dmaBusyOrFull)
}

// dmaWrite enqueues a DMA read command (rdram to dmem)
func DMARead(p []byte, addr cpu.Addr, n uint32) {
	cpu.WritebackSlice(p)
	rdramAddr := uintptr(unsafe.Pointer(unsafe.SliceData(p)))
	dma(rdramAddr, addr, n, dmaBusyOrFull)
}
