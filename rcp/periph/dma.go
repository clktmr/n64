package periph

import (
	"errors"
	"io"
	"unsafe"

	"github.com/clktmr/n64/debug"
	"github.com/clktmr/n64/rcp/cpu"
)

const (
	piBusStart = 0x0500_0000
	piBusEnd   = 0x1fbf_ffff
)

// Implememts io.ReadWriteSeeker for accessing devices on the PI bus.  It will
// automatically choose DMA transfers where alignment and cacheline padding
// allow it, otherwise fall back to copying via mmio.
//
// Devices must not have overlapping addresses, as they might cache bus
// accessses.
type Device struct {
	addr uint32
	size uint32
	seek int32 // TODO rename offset, make uint32

	// Caches WriteByte until a full word has been written.  This reduces PI
	// bus accesses but also helps to handle write-only devices more
	// gracefully.
	cache uint32
}

func NewDevice(piAddr uintptr, size uint32) *Device {
	addr := cpu.PhysicalAddress(piAddr)
	debug.Assert(addr >= piBusStart && addr+size <= piBusEnd, "invalid PI bus address")
	return &Device{addr, size, 0x0, 0}
}

var ErrSeekOutOfRange = errors.New("seek out of range")

func (v *Device) Addr() (piAddr uintptr) {
	return uintptr(v.addr)
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

	v.cacheWriteback()
	dmaStore(uintptr(dmaAddr), pdma)
	v.cacheInvalidate()

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
	v.cacheWriteback()
	dmaLoad(dmaAddr, pdma)

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
	v.cache = (v.cache &^ (0xff << shift)) | uint32(c)<<shift
	v.Seek(1, io.SeekCurrent)
	return nil
}

func (v *Device) ReadByte() (c byte, err error) {
	if uint32(v.seek) >= v.size {
		return 0, io.EOF
	}
	shift := (3 - v.seek%4) * 8
	c = byte(v.cache >> shift)
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
		cptr.Store(v.cache)
		v.cache = ncptr.Load()
	}
	return
}

func (v *Device) Flush() {
	v.cacheWriteback()
	waitDMA()
}

func (v *Device) assessTransfer(p []byte) (addr uintptr, dma []byte, head int, tail int) {
	dma, head, tail = cpu.PaddedSlice(p)
	if len(dma)&0x1 != 0 {
		// If DMA end address isn't 2 byte aligned, fallback to mmio for
		// the last byte.
		dma = dma[:len(dma)-1]
		tail += 1
	}
	addr = uintptr(int32(v.addr) + v.seek + int32(head))

	if addr&0x1 != 0 {
		// If DMA start address isn't 2 byte aligned there is no way to
		// use DMA at all, fallback to mmio for the whole transfer.
		tail += len(dma)
		dma = dma[:0]
	}

	return
}

func (v *Device) cacheTarget() *U32 {
	return (*U32)(unsafe.Pointer(cpu.KSEG1 | uintptr(v.addr+uint32(v.seek&^0x3))))
}

func (v *Device) cacheWriteback() {
	cptr := v.cacheTarget()
	cptr.Store(v.cache)
}

func (v *Device) cacheInvalidate() {
	cptr := v.cacheTarget()
	v.cache = cptr.Load()
}

// Loads bytes from PI bus into RDRAM via DMA
func dmaLoad(piAddr uintptr, p []byte) {
	if len(p) == 0 {
		return
	}

	addr := uintptr(unsafe.Pointer(unsafe.SliceData(p)))
	debug.Assert(piAddr%2 == 0, "PI start address unaligned")
	debug.Assert(len(p)%2 == 0, "PI end address unaligned")
	debug.Assert(cpu.IsPadded(p), "Unpadded destination slice")
	debug.Assert(addr%8 == 0, "RDRAM address unaligned")

	waitDMA()

	regs.dramAddr.Store(cpu.PhysicalAddress(addr))
	regs.cartAddr.Store(cpu.PhysicalAddress(piAddr))

	cpu.InvalidateSlice(p)

	n := len(p)
	regs.writeLen.Store(uint32(n - 1))

	waitDMA()
}

// Stores bytes from RDRAM to PI bus via DMA
func dmaStore(piAddr uintptr, p []byte) {
	if len(p) == 0 {
		return
	}

	addr := uintptr(unsafe.Pointer(unsafe.SliceData(p)))
	debug.Assert(piAddr%2 == 0, "PI start address unaligned")
	debug.Assert(len(p)%2 == 0, "PI end address unaligned")
	debug.Assert(cpu.IsPadded(p), "Unpadded destination slice")
	debug.Assert(addr%8 == 0, "RDRAM address unaligned")

	waitDMA()

	regs.dramAddr.Store(cpu.PhysicalAddress(addr))
	regs.cartAddr.Store(cpu.PhysicalAddress(piAddr))

	cpu.WritebackSlice(p)

	n := len(p)
	regs.readLen.Store(uint32(n - 1))
}

// Blocks until DMA has finished.
func waitDMA() {
	for {
		// TODO runtime.Gosched() ?
		if regs.status.Load()&(dmaBusy|ioBusy) == 0 {
			break
		}
	}

}

//go:nosplit
//go:nowritebarrierrec
func Handler() {
	regs.status.Store(clearInterrupt)
}
