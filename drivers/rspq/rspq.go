// Package rspq provides loading RSP microcodes developed for libdragon.
package rspq

import (
	"bytes"
	"embed"
	"embedded/mmio"
	"encoding/binary"
	"io"
	"slices"
	"unsafe"

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

const rspqDataAddress = 32

var rspqDataSize, rspqTextSize int

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

	Reset()
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
	rspqDataSize = len(uc.Data)
	rspqTextSize = len(uc.Text)

	lowpri.ClearBuffer(0)
	lowpri.ClearBuffer(1)
	highpri.ClearBuffer(0)
	highpri.ClearBuffer(1)
	lowpri.cur = 0
	highpri.cur = 0

	rspqData = rspQueue{}
	rspqData.RSPQDramLowpriAddr = cpu.PhysicalAddressSlice(lowpri.buffers[lowpri.bufIdx])
	rspqData.RSPQDramHighpriAddr = cpu.PhysicalAddressSlice(highpri.buffers[highpri.bufIdx])
	rspqData.RSPQDramAddr = rspqData.RSPQDramLowpriAddr
	rspqData.Tables.OverlayDescriptor[0].State = cpu.PhysicalAddressSlice(dummyOverlayState)
	rspqData.Tables.OverlayDescriptor[0].DataSize = uint16(len(dummyOverlayState) * 8)

	err = binary.Write(io.NewOffsetWriter(rsp.DMEM, rspqDataAddress), binary.BigEndian, &rspqData)
	if err != nil {
		panic(err)
	}

	var ovlhdr rspqOverlayHeader
	ovlhdr.Fields.StateStart = 0
	ovlhdr.Fields.StateSize = 7
	ovlhdr.Fields.CommandBase = 0

	err = binary.Write(io.NewOffsetWriter(rsp.DMEM, int64(rspqDataSize)), binary.BigEndian, &ovlhdr.Fields)
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
	if !rsp.Stopped() {
		return false
	}
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

	ctx.Append(1-ctx.bufIdx, CmdWriteStatus, uint32(ctx.bufdoneSig.SetMask()))
	ctx.Append(1-ctx.bufIdx, CmdJump, uint32(cpu.PhysicalAddressSlice(ctx.buffers[ctx.bufIdx])))

	rsp.Resume()
	ctx.cur = 0
}

var q rspQueue

// ucodes keeps a reference to registered rspq overlays to prevent them being
// garbage collected, since the RSP will need to load them whenever processing a
// command
var ucodes = make([]*ucode.UCode, 0, 8)
var pinner cpu.Pinner

func Register(p *ucode.UCode) (overlayId uint32) {
	r := bytes.NewReader(p.Data[rspqDataSize:])
	hdr, err := loadOverlayHeader(r)
	if err != nil {
		panic(err)
	}

	idx := slices.IndexFunc(q.Tables.OverlayDescriptor[1:], func(o overlayDescriptor) bool {
		return o.Code == 0
	})
	if idx == -1 {
		panic("max overlay count")
	}
	idx += 1

	slotCount := (len(hdr.Commands) + 15) >> 4

	id := bytes.Index(q.Tables.OverlayTable[1:], make([]byte, slotCount))
	if id == -1 {
		panic("max command count")
	}
	id += 1

	desc := &rspqData.Tables.OverlayDescriptor[idx]
	code := p.Text[rspqTextSize:]
	data := p.Data[rspqDataSize:]
	state := p.Data[hdr.Fields.StateStart : hdr.Fields.StateStart+hdr.Fields.StateSize]
	desc.Code = cpu.PhysicalAddressSlice(code)
	desc.Data = cpu.PhysicalAddressSlice(data)
	desc.CodeSize = uint16(len(code))
	desc.DataSize = uint16(len(data))
	desc.State = cpu.PhysicalAddressSlice(state)

	// Let the assigned ids point at the overlay
	for i := range slotCount {
		rspqData.Tables.OverlayTable[id+i] = uint8(idx * int(unsafe.Sizeof(rspqData.Tables.OverlayDescriptor[0])))
	}
	hdr.Fields.CommandBase = uint16(id << 5)
	err = hdr.Store(bytes.NewBuffer(p.Data[rspqDataSize:rspqDataSize]))
	if err != nil {
		panic(err)
	}

	pinner.Pin(unsafe.SliceData(p.Text))
	pinner.Pin(unsafe.SliceData(p.Data))
	ucodes = append(ucodes, p)

	cpu.WritebackSlice(p.Text)
	cpu.WritebackSlice(p.Data)

	// TODO let rsp do the dma, so we don't have to wait
	for !rsp.Stopped() {
		// wait
	}
	err = binary.Write(io.NewOffsetWriter(rsp.DMEM, rspqDataAddress), binary.BigEndian, &rspqData)
	if err != nil {
		panic(err)
	}

	return uint32(id << 28)
}

const (
	maxOverlayCount        = 8
	overlayIdCount         = 16
	maxOverlayCommandCount = ((maxOverlayCount - 1) * 16)
)
