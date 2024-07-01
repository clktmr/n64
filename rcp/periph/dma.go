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
// allow it, otherwise fall back to copying via CPU load/store.
type Device struct {
	addr uint32
	size uint32
	seek int32
}

func NewDevice(piAddr uintptr, size uint32) *Device {
	debug.Assert(piAddr%2 == 0, "PI start address unaligned")
	addr := cpu.PhysicalAddress(piAddr)
	debug.Assert(addr >= piBusStart && addr+size <= piBusEnd, "invalid PI bus address")
	return &Device{addr, size, 0x0}
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

	pdma, head, tail := cpu.PaddedSlice(p)
	dmaStore(uintptr(int32(v.addr)+v.seek), pdma)

	for i := range head {
		v.WriteByte(p[i])
	}

	v.seek += int32(len(pdma))

	tailBase := head + len(pdma)
	for i := range tail {
		v.WriteByte(p[tailBase+i])
	}
	return
}

func (v *Device) Read(p []byte) (n int, err error) {
	n = len(p)
	left := int(v.size) - int(v.seek)
	if n > left {
		n = left
		p = p[:left]
		err = io.EOF
	}
	n = min(int(int32(v.size)-v.seek), len(p))

	pdma, head, tail := cpu.PaddedSlice(p)
	dmaLoad(uintptr(int32(v.addr)+v.seek), pdma)

	for i := range head {
		p[i], _ = v.ReadByte()
	}

	v.seek += int32(len(pdma))

	tailBase := head + len(pdma)
	for i := range tail {
		p[tailBase+i], _ = v.ReadByte()
	}
	v.seek += int32(n)
	return
}

func (v *Device) WriteByte(c byte) error {
	if uint32(v.seek) >= v.size {
		return io.ErrShortWrite
	}
	cptr, shift := v.byteMask()
	v.seek += 1
	cptr.StoreBits(0xff<<shift, uint32(c)<<shift)
	return nil
}

func (v *Device) ReadByte() (byte, error) {
	if uint32(v.seek) >= v.size {
		return 0, io.EOF
	}
	cptr, shift := v.byteMask()
	v.seek += 1
	return byte(cptr.LoadBits(0xff<<shift) >> shift), nil
}

func (v *Device) byteMask() (cptr *U32, shift int32) {
	cptr = (*U32)(unsafe.Pointer(cpu.KSEG1 | uintptr(v.addr+uint32(v.seek&^0x3))))
	shift = (3 - v.seek%4) * 8
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
	waitDMA()
}

// Loads bytes from PI bus into RDRAM via DMA
func dmaLoad(piAddr uintptr, p []byte) {
	if len(p) == 0 {
		return
	}

	addr := uintptr(unsafe.Pointer(unsafe.SliceData(p)))
	debug.Assert(cpu.IsPadded(p), "Unpadded destination slice")
	debug.Assert(addr%8 == 0, "RDRAM address unaligned")

	regs.dramAddr.Store(cpu.PhysicalAddress(addr))
	regs.cartAddr.Store(cpu.PhysicalAddress(piAddr))

	cpu.InvalidateSlice(p)

	n := len(p)
	regs.writeLen.Store(uint32(n + n%2 - 1))

	waitDMA()
}

// Stores bytes from RDRAM to PI bus via DMA
func dmaStore(piAddr uintptr, p []byte) {
	if len(p) == 0 {
		return
	}

	addr := uintptr(unsafe.Pointer(unsafe.SliceData(p)))
	debug.Assert(addr%8 == 0, "RDRAM address unaligned")

	regs.dramAddr.Store(cpu.PhysicalAddress(addr))
	regs.cartAddr.Store(cpu.PhysicalAddress(piAddr))

	cpu.WritebackSlice(p)

	n := len(p)
	regs.readLen.Store(uint32(n + n%2 - 1))
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
