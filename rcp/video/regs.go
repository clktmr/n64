package video

import (
	"embedded/mmio"
	"unsafe"
)

var regs *registers = (*registers)(unsafe.Pointer(BASE_ADDR))

const BASE_ADDR = uintptr(0xffffffffa440_0000)

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
	BBP16 ColorDepth = 2
	BBP32 ColorDepth = 3
)

func SetupNTSC(bbp ColorDepth) {
	regs.control.Store(uint32(bbp))
	regs.width.Store(320)
	regs.vInt.Store(0x200)
	regs.currentLine.Store(352)
	regs.timing.Store(0x3e5_2239)
	regs.vSync.Store(525)
	regs.hSync.Store((0 << 16) | 3093)
	regs.hSync2.Store((3093 << 16) | 3093)
	regs.hLimits.Store((108 << 16) | 748)
	regs.vLimits.Store((37 << 16) | 511)
	regs.colorBurst.Store((14 << 16) | 516)
	regs.hScale.Store((0 << 16) | 512)
	regs.vScale.Store((0 << 16) | 1024)
}

func SetFramebuffer(addr uintptr) {
	regs.framebuffer.Store(uint32(addr))
}
