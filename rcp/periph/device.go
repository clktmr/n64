// Package periph provides IO and DMA on the PI bus.
package periph

import (
	"embedded/rtos"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/clktmr/n64/debug"
	"github.com/clktmr/n64/rcp/cpu"
)

const (
	piBus0Start = 0x0500_0000
	piBus0End   = 0x1fbf_ffff
	piBus1Start = 0x1fd0_0000
	piBus1End   = 0x7fff_ffff
)

var ErrEndOfDevice = errors.New("end of device")

// Device implememts io.ReaderAt and io.WriterAt for accessing devices on the PI
// bus. It will automatically choose DMA transfers where alignment and cacheline
// padding allow it, otherwise fall back to copying via mmio.
type Device struct {
	addr cpu.Addr
	size uint32

	done *rtos.Cond
	mtx  sync.Mutex
}

func NewDevice(piAddr cpu.Addr, size uint32) *Device {
	addr := uint32(piAddr)
	debug.Assert((addr >= piBus0Start && addr+size <= piBus0End) ||
		(addr >= piBus1Start && addr+size <= piBus1End),
		"invalid pi bus address")
	return &Device{addr: piAddr, size: size, done: allocCond()}
}

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

	addr := uintptr(unsafe.Pointer(unsafe.SliceData(p)))
	dmaSync(addr, dmaJob{v.addr + cpu.Addr(off), p, dmaLoad, v.done})
	n = len(p)

	return
}

func (v *Device) WriteAt(p []byte, off int64) (n int, err error) {
	v.mtx.Lock()
	defer v.mtx.Unlock()

	left := int(v.size) - int(off)
	if len(p) > left {
		p = p[:left]
		err = ErrEndOfDevice
	}

	addr := uintptr(unsafe.Pointer(unsafe.SliceData(p)))
	dmaSync(addr, dmaJob{v.addr + cpu.Addr(off), p, dmaStore, v.done})
	n = len(p)

	return
}

//go:uintptrescapes
func dmaSync(_ uintptr, job dmaJob) {
	dma(job)
	if !job.done.Wait(1 * time.Second) {
		panic("dma timeout")
	}
}

var (
	condPool    [64]rtos.Cond
	condPoolIdx atomic.Int32
)

func allocCond() *rtos.Cond {
	return &condPool[condPoolIdx.Add(1)-1]
}
