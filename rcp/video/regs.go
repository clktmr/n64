// Video DAC which reads an image from RDRAM and outputs it to screen as either
// NTSC, PAL or M-PAL.
package video

import (
	"embedded/mmio"
	"unsafe"

	"github.com/clktmr/n64/debug"
	"github.com/clktmr/n64/rcp/cpu"
	"github.com/clktmr/n64/rcp/texture"
)

var regs *registers = (*registers)(unsafe.Pointer(baseAddr))

const baseAddr uintptr = cpu.KSEG1 | 0x0440_0000

type registers struct {
	control   mmio.U32
	origin    mmio.U32
	width     mmio.U32
	vIntr     mmio.U32
	vCurrent  mmio.U32
	burst     mmio.U32
	vSync     mmio.U32
	hSync     mmio.U32
	hSyncLeap mmio.U32
	hVideo    mmio.U32
	vVideo    mmio.U32
	vBurst    mmio.U32
	xScale    mmio.U32
	yScale    mmio.U32
}

type ColorDepth uint32

const (
	BPP16 ColorDepth = 2
	BPP32 ColorDepth = 3
)

func SetupNTSC(fb texture.Texture) {
	// Avoid crash by disabling output while changing registers
	regs.control.Store(0)

	regs.burst.Store(0x3e5_2239)
	regs.vSync.Store(0x20d)
	regs.hSync.Store(0x0c15)
	regs.hSyncLeap.Store(0x0c15_0c15)
	regs.hVideo.Store(0x006c_02ec)
	regs.vVideo.Store(0x0025_01ff)
	regs.vBurst.Store(0x000e_0204)

	setupCommon(fb)
}

func SetupPAL(fb texture.Texture) {
	// Avoid crash by disabling output while changing registers
	regs.control.Store(0)

	regs.burst.Store(0x0404_233a)
	regs.vSync.Store(0x271)
	regs.hSync.Store(0x0015_0c69)
	regs.hSyncLeap.Store(0x0c6f_0c6e)
	regs.hVideo.Store(0x0080_0300)
	regs.vVideo.Store(0x005f_0239)
	regs.vBurst.Store(0x0009_026b)

	setupCommon(fb)
}

func setupCommon(fb texture.Texture) {
	SetFrambuffer(fb)

	width, height := uint32(fb.Bounds().Dx()), uint32(fb.Bounds().Dy())
	regs.width.Store(width)
	regs.vIntr.Store(2)

	regs.xScale.Store((1024*width + 320) / 640)
	regs.yScale.Store((1024*height + 120) / 240)

	regs.control.Store(uint32(bpp(fb.BPP())) | (3 << 8))
}

func SetFrambuffer(fb texture.Texture) {
	regs.origin.Store(cpu.PhysicalAddress(fb.Addr()))
}

func bpp(bpp texture.BitDepth) ColorDepth {
	switch bpp {
	case texture.BBP16:
		return BPP16
	case texture.BBP32:
		return BPP32
	default:
		debug.Assert(false, "video: unsupported framebuffer format")
	}
	return 0
}
