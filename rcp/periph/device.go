package periph

import (
	"embedded/rtos"
	"errors"
	"io"
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

// Implememts io.ReadWriteSeeker for accessing devices on the PI bus.  It will
// automatically choose DMA transfers where alignment and cacheline padding
// allow it, otherwise fall back to copying via mmio.
//
// Devices must not have overlapping addresses, as they might cache bus
// accessses.
type Device struct {
	addr cpu.Addr
	size uint32
	seek int32 // TODO rename offset, make uint32

	// Caches WriteByte until a full word has been written.  This reduces PI
	// bus accesses but also helps to handle write-only devices more
	// gracefully.  These would otherwise read a dword and write it back on
	// each byte write, which breaks if the read op returns garbage.
	cache uint32
	valid bool

	flushed *rtos.Note
}

func NewDevice(piAddr cpu.Addr, size uint32) *Device {
	addr := uint32(piAddr)
	debug.Assert((addr >= piBus0Start && addr+size <= piBus0End) ||
		(addr >= piBus1Start && addr+size <= piBus1End),
		"invalid pi bus address")
	return &Device{piAddr, size, 0x0, 0, false, nil}
}

var ErrSeekOutOfRange = errors.New("seek out of range")

func (v *Device) Addr() cpu.Addr {
	return v.addr
}

func (v *Device) Size() int {
	return int(v.size)
}

func (v *Device) Write(p []byte) (n int, err error) {
	n = len(p)
	left := int(v.size) - int(v.seek)
	if n > left {
		n = left
		p = p[:left]
		err = io.ErrShortWrite
	}

	dmaAddr, pdma, head, tail := v.assessTransfer(p)

	for i := range head {
		v.WriteByte(p[i])
	}

	v.Seek(int64(len(pdma)), io.SeekCurrent)

	tailBase := head + len(pdma)
	for i := range tail {
		v.WriteByte(p[tailBase+i])
	}

	if len(pdma) > 0 {
		v.cacheWriteback()
		v.flushed = dma(dmaAddr, pdma, dmaStore)
		v.cacheInvalidate()
	}

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

	dmaAddr, pdma, head, tail := v.assessTransfer(p)

	// Do the DMA before the mmio because it might invalidate parts of head
	// and tail
	if len(pdma) > 0 {
		v.cacheWriteback()
		flushed := dma(dmaAddr, pdma, dmaLoad)
		if !flushed.Sleep(1 * time.Second) {
			panic("dma queue timeout")
		}
	}

	for i := range head {
		p[i], _ = v.ReadByte()
	}

	v.Seek(int64(len(pdma)), io.SeekCurrent)

	tailBase := head + len(pdma)
	for i := range tail {
		p[tailBase+i], _ = v.ReadByte()
	}

	return
}

func (v *Device) WriteByte(c byte) error {
	if uint32(v.seek) >= v.size {
		return io.ErrShortWrite
	}
	shift := (3 - v.seek%4) * 8
	v.cache = (v.cacheRead() &^ (0xff << shift)) | uint32(c)<<shift
	v.Seek(1, io.SeekCurrent)
	return nil
}

func (v *Device) ReadByte() (c byte, err error) {
	if uint32(v.seek) >= v.size {
		return 0, io.EOF
	}
	shift := (3 - v.seek%4) * 8
	c = byte(v.cacheRead() >> shift)
	v.Seek(1, io.SeekCurrent)
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

	cptr := v.cacheTarget()
	v.seek = int32(newoffset)

	ncptr := v.cacheTarget()
	if cptr != ncptr {
		if v.valid {
			cptr.Store(v.cache)
		}
		v.cacheInvalidate()
	}
	return
}

func (v *Device) Flush() {
	v.cacheWriteback()
	if v.flushed != nil && !v.flushed.Sleep(1*time.Second) {
		panic("dma queue timeout")
	}
}

func (v *Device) assessTransfer(p []byte) (addr cpu.Addr, dma []byte, head int, tail int) {
	dma, head, tail = cpu.PaddedSlice(p)
	if len(dma)&0x1 != 0 {
		// If DMA end address isn't 2 byte aligned, fallback to mmio for
		// the last byte.
		dma = dma[:len(dma)-1]
		tail += 1
	}
	addr = v.addr + cpu.Addr(v.seek) + cpu.Addr(head)

	if addr&0x1 != 0 {
		// If DMA start address isn't 2 byte aligned there is no way to
		// use DMA at all, fallback to mmio for the whole transfer.
		tail += len(dma)
		dma = dma[:0]
	}

	return
}

func (v *Device) cacheTarget() *U32 {
	return (*U32)(unsafe.Pointer(cpu.KSEG1 | uintptr(int32(v.addr)+(v.seek&^0x3))))
}

func (v *Device) cacheWriteback() {
	if v.valid {
		v.cacheTarget().Store(v.cache)
	}
}

func (v *Device) cacheInvalidate() {
	v.valid = false
}

// Writes back and invalidates the single dword cache of this device.  Call this
// before another component writes to the device.
func (v *Device) WritebackInvalidate() {
	v.cacheWriteback()
	v.cacheInvalidate()
}

func (v *Device) cacheRead() uint32 {
	if v.valid == false {
		v.cache = v.cacheTarget().Load()
		v.valid = true
	}
	return v.cache
}
