package rspq

import (
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

func dma(p []byte, dmemAddr cpu.Addr, flags uint32) {
	debug.Assert(dmemAddr&0x7 == 0 && len(p)&0x7 == 0, "unaligned dma")
	Write(CmdDma, uint32(cpu.PhysicalAddressSlice(p)), uint32(dmemAddr), uint32(len(p)-1), flags)
}

const dmaBusyOrFull = 12

// dmaWrite enqueues a DMA write command (dmem to rdram)
func DMAWrite(p []byte, addr cpu.Addr) {
	cpu.InvalidateSlice(p)
	dma(p, addr, 0xffff_8000|dmaBusyOrFull)
}

// dmaWrite enqueues a DMA read command (rdram to dmem)
func DMARead(p []byte, addr cpu.Addr) {
	cpu.WritebackSlice(p)
	dma(p, addr, dmaBusyOrFull)
}
