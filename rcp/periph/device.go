package periph

import (
	"embedded/rtos"
	"errors"
	"io"
	"sync"
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

// Device implememts io.ReaderAt and io.WriterAt for accessing devices on the PI
// bus.  It will automatically choose DMA transfers where alignment and
// cacheline padding allow it, otherwise fall back to copying via mmio.
type Device struct {
	addr cpu.Addr
	size uint32

	done rtos.Note
	mtx  sync.Mutex
}

func NewDevice(piAddr cpu.Addr, size uint32) *Device {
	addr := uint32(piAddr)
	debug.Assert((addr >= piBus0Start && addr+size <= piBus0End) ||
		(addr >= piBus1Start && addr+size <= piBus1End),
		"invalid pi bus address")
	return &Device{addr: piAddr, size: size}
}

var ErrSeekOutOfRange = errors.New("seek out of range")

func (v *Device) Addr() cpu.Addr {
	return v.addr
}

func (v *Device) Size() int {
	return int(v.size)
}

func (v *Device) ReadAt(p []byte, off int64) (n int, err error) {
	v.mtx.Lock()
	defer v.mtx.Unlock()

	left := int(v.size) - int(off)
	if len(p) >= left {
		p = p[:left]
		err = io.EOF
	}

	v.done.Clear()
	dma(dmaJob{v.addr + cpu.Addr(off), p, dmaLoad, &v.done})
	if !v.done.Sleep(1 * time.Second) {
		panic("dma timeout")
	}
	n = len(p)

	return
}

func (v *Device) WriteAt(p []byte, off int64) (n int, err error) {
	v.mtx.Lock()
	defer v.mtx.Unlock()

	left := int(v.size) - int(off)
	if len(p) > left {
		p = p[:left]
		err = io.ErrShortWrite
	}

	v.done.Clear()
	dma(dmaJob{v.addr + cpu.Addr(off), p, dmaStore, &v.done})
	if !v.done.Sleep(1 * time.Second) {
		panic("dma timeout")
	}
	n = len(p)

	return
}
