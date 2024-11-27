package periph

import (
	"embedded/rtos"
	"errors"
	"io"
	"time"

	"github.com/clktmr/n64/debug"
	"github.com/clktmr/n64/rcp/cpu"
)

const (
	piBus0Start = 0x0500_0000
	piBus0End   = 0x1fbf_ffff
	piBus1Start = 0x1fd0_0000
	piBus1End   = 0x7fff_ffff
)

// Implememts io.ReadWriteSeeker for accessing devices on the PI bus.  It will
// automatically choose DMA transfers where alignment and cacheline padding
// allow it, otherwise fall back to copying via mmio.
type Device struct {
	addr cpu.Addr
	size uint32
	seek int32 // TODO rename offset, make uint32

	flushed *rtos.Note
}

func NewDevice(piAddr cpu.Addr, size uint32) *Device {
	addr := uint32(piAddr)
	debug.Assert((addr >= piBus0Start && addr+size <= piBus0End) ||
		(addr >= piBus1Start && addr+size <= piBus1End),
		"invalid pi bus address")
	return &Device{piAddr, size, 0x0, nil}
}

var ErrSeekOutOfRange = errors.New("seek out of range")

func (v *Device) Addr() cpu.Addr {
	return v.addr
}

func (v *Device) Size() int {
	return int(v.size)
}

// FIXME must not retain p
func (v *Device) Write(p []byte) (n int, err error) {
	n = len(p)
	left := int(v.size) - int(v.seek)
	if n > left {
		n = left
		p = p[:left]
		err = io.ErrShortWrite
	}

	v.flushed = dma(v.addr+cpu.Addr(v.seek), p, dmaStore)

	v.Seek(int64(n), io.SeekCurrent)

	return
}

func (v *Device) Read(p []byte) (n int, err error) {
	n = len(p)
	left := int(v.size) - int(v.seek)
	if n >= left {
		n = left
		p = p[:left]
		err = io.EOF
	}

	done := dma(v.addr+cpu.Addr(v.seek), p, dmaLoad)
	if done != nil && !done.Sleep(1*time.Second) {
		panic("dma queue timeout")
	}

	v.Seek(int64(n), io.SeekCurrent)
	return
}

func (v *Device) Seek(offset int64, whence int) (newoffset int64, err error) {
	switch whence {
	case io.SeekStart:
		// newoffset = 0
	case io.SeekCurrent:
		newoffset += int64(v.seek)
	case io.SeekEnd:
		newoffset = int64(v.size)
	}
	newoffset += offset
	if newoffset < 0 || newoffset > int64(v.size) {
		return int64(v.seek), ErrSeekOutOfRange
	}

	v.seek = int32(newoffset)

	return
}

func (v *Device) Flush() {
	if v.flushed != nil && !v.flushed.Sleep(1*time.Second) {
		panic("dma queue timeout")
	}
	v.flushed = nil
}
