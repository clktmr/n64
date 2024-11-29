package periph

import (
	"embedded/rtos"
	"errors"
	"io"
	"sync"

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
//
// Device is safe for concurrent use.
type Device struct {
	addr cpu.Addr
	size uint32
	seek int32 // TODO rename offset, make uint32

	wJodId uint64
	done   rtos.Note
	mtx    sync.Mutex
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

	n = len(p)
	id := dma(dmaJob{v.addr + cpu.Addr(off), p, dmaLoad, nil})
	flush(id, &v.done)

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

	for len(p) > 0 {
		buf, _ := getBuf()
		nn := copy(buf, p)
		p = p[nn:]
		v.wJodId = dma(dmaJob{v.addr + cpu.Addr(off), buf[:nn], dmaStore, nil})

		n += nn
	}

	return
}

func (v *Device) Write(p []byte) (n int, err error) {
	v.mtx.Lock()
	defer v.mtx.Unlock()

	left := int(v.size) - int(v.seek)
	if len(p) > left {
		p = p[:left]
		err = io.ErrShortWrite
	}

	for len(p) > 0 {
		buf, _ := getBuf()
		nn := copy(buf, p)
		p = p[nn:]
		v.wJodId = dma(dmaJob{v.addr + cpu.Addr(v.seek), buf[:nn], dmaStore, nil})

		n += nn
	}

	v.Seek(int64(n), io.SeekCurrent)

	return
}

func (v *Device) Read(p []byte) (n int, err error) {
	v.mtx.Lock()
	defer v.mtx.Unlock()

	n = len(p)
	left := int(v.size) - int(v.seek)
	if n >= left {
		n = left
		p = p[:left]
		err = io.EOF
	}

	id := dma(dmaJob{v.addr + cpu.Addr(v.seek), p, dmaLoad, nil})
	flush(id, &v.done)

	v.Seek(int64(n), io.SeekCurrent)
	return
}

func (v *Device) Seek(offset int64, whence int) (newoffset int64, err error) {
	v.mtx.Lock()
	defer v.mtx.Unlock()

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
	v.mtx.Lock()
	defer v.mtx.Unlock()

	flush(v.wJodId, &v.done)
}
