package rspq

import (
	"encoding/binary"
	"io"

	"github.com/clktmr/n64/rcp/cpu"
	"github.com/clktmr/n64/rcp/rsp"
)

const (
	MaxCommandSize      = 62
	MaxShortCommandSize = 16
)

var (
	rspqData rspQueue

	ctx     = lowpri
	lowpri  = newContext(0x200, sigBufdoneLow)
	highpri = newContext(0x80, sigBufdoneHigh)

	dummyOverlayState = cpu.MakePaddedSlice[uint64](2)
)

func newContext(bufsize int, signal rsp.Signal) *context {
	ctx := &context{bufdoneSig: signal}
	for i := range ctx.buffers {
		ctx.buffers[i] = cpu.MakePaddedSlice[uint32](bufsize)
	}
	return ctx
}

// This struct isn't known by the rsp_queue microcode.
// See rspq_ctx_t in libdragons's rspq.c
type context struct {
	buffers    [2][]uint32
	bufIdx     int
	bufdoneSig rsp.Signal
	cur        int
}

func (p *context) ClearBuffer(idx int) {
	buffer := cpu.UncachedSlice(p.buffers[idx])
	clear(buffer)
}

type overlayDescriptor struct {
	Code, Data, State  cpu.Addr
	CodeSize, DataSize uint16
}

// Struct layout is known by rsp_queue microcode and copied to DMEM. See
// rsp_queue_s in libdragons's rspq_internal.h
type rspQueue struct {
	Tables struct {
		OverlayTable      [0x10]uint8
		OverlayDescriptor [8]overlayDescriptor
	}
	RSPQPointerStack    [8]uint32
	RSPQDramLowpriAddr  cpu.Addr
	RSPQDramHighpriAddr cpu.Addr
	RSPQDramAddr        cpu.Addr
	RSPQRdpSentinel     uint32
	RSPQRdpMode         struct {
		Combiner               uint64
		CombinerMipMapMask     uint64
		BlendStep0, BlendStep1 uint32
		OtherModes             uint64
	}
	RDPScissorRect     uint64
	RSPQRdpBuffers     [2]cpu.Addr
	RSPQRdpCurrent     uint32
	RDPFillColor       uint32
	RDPTargetBitdepth  uint8
	RDPSyncfullOngoing uint8
	RDPQDebug          uint8
	_                  uint8
	CurrentOvl         int16
}

// Struct layout is known by rsp_queue microcode and copied to DMEM.
type rspqOverlayHeader struct {
	Fields struct {
		StateStart  uint16 // Start of the portion of DMEM used as "state"
		StateSize   uint16 // Size of the portion of DMEM used as "state"
		CommandBase uint16 // Primary overlay ID used for this overlay
		_           uint16
	}
	Commands []uint16
}

func loadOverlayHeader(r io.Reader) (*rspqOverlayHeader, error) {
	p := &rspqOverlayHeader{}
	err := binary.Read(r, binary.BigEndian, &p.Fields)
	if err != nil {
		return nil, err
	}
	p.Commands = cpu.MakePaddedSlice[uint16](maxOverlayCommandCount)
	for i := range p.Commands {
		err = binary.Read(r, binary.BigEndian, &p.Commands[i])
		if err != nil {
			return nil, err
		}
		if p.Commands[i] == 0 {
			p.Commands = p.Commands[:i]
			break
		}
	}
	return p, nil
}

func (p *rspqOverlayHeader) Store(w io.Writer) error {
	err := binary.Write(w, binary.BigEndian, &p.Fields)
	if err != nil {
		return err
	}
	err = binary.Write(w, binary.BigEndian, append(p.Commands, 0))
	if err != nil {
		return err
	}
	return nil
}
