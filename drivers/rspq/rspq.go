// Package rspq provides loading RSP microcodes developed for libdragon.
package rspq

import (
	"embed"
	"embedded/mmio"
	"encoding/binary"
	"io"

	"github.com/clktmr/n64/debug"
	"github.com/clktmr/n64/drivers/cartfs"
	"github.com/clktmr/n64/rcp/cpu"
	"github.com/clktmr/n64/rcp/rsp"
	"github.com/clktmr/n64/rcp/rsp/ucode"
)

var (
	// rsp_queue microcode from libdragon
	// Version: 3feaaadf0 (RSPQ_DEBUG enabled)
	//
	//go:embed rsp_queue.ucode rsp_crash.ucode
	_rspQueueFiles embed.FS
	rspQueueFiles  cartfs.FS = cartfs.Embed(_rspQueueFiles)
)

const (
	rspqDataAddress    = 32
	overlayDataAddress = 0x260 // TODO generated at link time
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

const (
	_                   = 1 << iota
	sigRdpsyncfull      // Signal used by RDP SYNC_FULL command to notify that an interrupt is pending
	sigSyncpoint        // Signal used by RSP to notify that a syncpoint was reached
	sigHighpriRunning   // Signal used to notify that RSP is executing the highpri queue
	sigHighpriRequested // Signal used to notify that the CPU has requested that the RSP switches to the highpri queue
	sigBufdoneHigh      // Signal used by RSP to notify that has finished one of the two buffers of the highpri queue
	sigBufdoneLow       // Signal used by RSP to notify that has finished one of the two buffers of the lowpri queue
	sigMore             // Signal used by the CPU to notify the RSP that more data has been written in the current queue
)

func init() {
	cpu.WritebackSlice(lowpri.buffers[0])
	cpu.WritebackSlice(lowpri.buffers[1])
	cpu.WritebackSlice(highpri.buffers[0])
	cpu.WritebackSlice(highpri.buffers[1])
	cpu.WritebackSlice(dummyOverlayState[:])
}

func Reset() {
	r, err := rspQueueFiles.Open("rsp_queue.ucode")
	if err != nil {
		panic(err)
	}
	uc, err := ucode.Load(r)
	if err != nil {
		panic(err)
	}

	rsp.Load(uc)

	lowpri.ClearBuffer(0)
	lowpri.ClearBuffer(1)
	highpri.ClearBuffer(0)
	highpri.ClearBuffer(1)
	lowpri.cur = 0
	highpri.cur = 0

	var hdr rspQueue
	hdr.RSPQDramLowpriAddr = cpu.PhysicalAddressSlice(lowpri.buffers[lowpri.bufIdx])
	hdr.RSPQDramHighpriAddr = cpu.PhysicalAddressSlice(highpri.buffers[highpri.bufIdx])
	hdr.RSPQDramAddr = hdr.RSPQDramLowpriAddr
	hdr.Tables.OverlayDescriptor[0].State = cpu.PhysicalAddressSlice(dummyOverlayState)
	hdr.Tables.OverlayDescriptor[0].DataSize = uint16(len(dummyOverlayState) * 8)

	err = binary.Write(io.NewOffsetWriter(rsp.DMEM, rspqDataAddress), binary.BigEndian, &hdr)
	if err != nil {
		panic(err)
	}

	var ovlhdr rspqOverlayHeader
	ovlhdr.StateStart = 0
	ovlhdr.StateSize = 7
	ovlhdr.CommandBase = 0

	err = binary.Write(io.NewOffsetWriter(rsp.DMEM, overlayDataAddress), binary.BigEndian, &ovlhdr)
	if err != nil {
		panic(err)
	}

	rsp.ClearSignals(0xff)
	rsp.SetSignals(sigBufdoneLow | sigBufdoneHigh)

	rsp.Resume()
}

func (p *context) Append(bufidx int, c Command, args ...uint32) {
	buffer := cpu.UncachedSlice(p.buffers[bufidx])

	if len(args) == 0 {
		buffer[p.cur] = (uint32(c) << 24)
		p.cur++
	} else if args != nil {
		debug.Assert(args[0]&0xff000000 == 0, "invalid command")

		// cmd byte must be written last
		copy(buffer[p.cur+1:], args[1:])
		buffer[p.cur] = args[0] | (uint32(c) << 24)
		p.cur += len(args)
	}
}

// TODO should be implemented in assembly for performance
func Write(c Command, args ...uint32) {
	ctx.Append(ctx.bufIdx, c, args...)

	rsp.SetSignals(sigMore)
	rsp.Resume()

	if ctx.cur+MaxCommandSize > len(ctx.buffers[ctx.bufIdx]) {
		nextBuffer()
	}
}

func Crashed() bool {
	if !rsp.Halted() {
		return false
	}
	// FIXME don't use mmio if dma is busy
	imem := cpu.MMIO[[1024]mmio.U32](cpu.Addr(rsp.IMEM))
	instruction := imem[(rsp.PC()>>2)+1].Load()
	return instruction == 0x00ba000d // halted in break loop
}

// switchContexts switches between low and high priority context
func switchContexts(newctx *context) {
	ctx = newctx
}

// nextBuffer switches to the other buffer of the current context
func nextBuffer() {
	// TODO if block { ... }
	for rsp.Signals()&ctx.bufdoneSig == 0 {
		// wait
		rsp.Resume()
	}
	rsp.ClearSignals(ctx.bufdoneSig)
	ctx.bufIdx = 1 - ctx.bufIdx

	ctx.ClearBuffer(ctx.bufIdx)

	ctx.Append(1-ctx.bufIdx, CmdWriteStatus, rsp.SetSignalsMask(ctx.bufdoneSig))
	ctx.Append(1-ctx.bufIdx, CmdJump, uint32(cpu.PhysicalAddressSlice(ctx.buffers[ctx.bufIdx])))

	rsp.Resume()
	ctx.cur = 0
}
