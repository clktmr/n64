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

const (
	// TODO support other resolutions
	WIDTH  = 320
	HEIGHT = 240
)

var regs *registers = (*registers)(unsafe.Pointer(baseAddr))

const baseAddr uintptr = cpu.KSEG1 | 0x0440_0000

type registers struct {
	control     mmio.U32
	framebuffer mmio.U32
	width       mmio.U32
	vInt        mmio.U32
	currentLine mmio.U32
	timing      mmio.U32
	vSync       mmio.U32
	hSync       mmio.U32
	hSync2      mmio.U32
	hLimits     mmio.U32
	vLimits     mmio.U32
	colorBurst  mmio.U32
	hScale      mmio.U32
	vScale      mmio.U32
}

type ColorDepth uint32

const (
	BPP16 ColorDepth = 2
	BPP32 ColorDepth = 3
)

func SetupNTSC(fb texture.Texture) {
	// Avoid crash by disabling output while changing registers
	regs.control.Store(0)

	regs.width.Store(WIDTH)
	regs.vInt.Store(2)
	regs.currentLine.Store(0)
	regs.timing.Store(0x3e5_2239)
	regs.vSync.Store(0x20d)
	regs.hSync.Store(0x0c15)
	regs.hSync2.Store(0x0c15_0c15)
	regs.hLimits.Store(0x006c_02ec)
	regs.vLimits.Store(0x0025_01ff)
	regs.colorBurst.Store(0x000e_0204)
	regs.hScale.Store((1024*WIDTH + WIDTH) / 640)
	regs.vScale.Store((1024*HEIGHT + 120) / HEIGHT)

	regs.control.Store(uint32(bpp(fb.BPP())) | (3 << 8))
}

func SetupPAL(fb texture.Texture) {
	// Avoid crash by disabling output while changing registers
	regs.control.Store(0)

	regs.framebuffer.Store(uint32(fb.Addr()))
	regs.width.Store(WIDTH)
	regs.vInt.Store(2)
	regs.currentLine.Store(0)
	regs.timing.Store(0x0404_233a)
	regs.vSync.Store(0x271)
	regs.hSync.Store(0x0015_0c69)
	regs.hSync2.Store(0x0c6f_0c6e)
	regs.hLimits.Store(0x0080_0300)
	regs.vLimits.Store(0x005f_0239)
	regs.colorBurst.Store(0x0009_026b)
	regs.hScale.Store((1024*WIDTH + WIDTH) / 640)
	regs.vScale.Store((1024*HEIGHT + 120) / HEIGHT)

	regs.control.Store(uint32(bpp(fb.BPP())) | (3 << 8))
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
