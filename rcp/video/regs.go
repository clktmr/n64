// Video DAC which reads an image from RDRAM and outputs it to screen as either
// NTSC, PAL or M-PAL.
package video

import (
	"embedded/mmio"
	"image"
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

type AntiAliasMode uint32

const (
	AANone       AntiAliasMode = 3 << 8
	AAResampling AntiAliasMode = 2 << 8
	AAEnabled    AntiAliasMode = 1 << 8
	AADedither   AntiAliasMode = 0 << 8
)

type ControlFlags uint32

const (
	ControlDedither    ControlFlags = 1 << 16
	ControlSerrate     ControlFlags = 1 << 6
	ControlDivot       ControlFlags = 1 << 4
	ControlGamma       ControlFlags = 1 << 3
	ControlGammaDither ControlFlags = 1 << 2
)

var (
	Interlaced bool

	framebuffer texture.Texture

	limits     image.Rectangle
	limitsNTSC = image.Rect(0, 0, 640, 480).Add(image.Point{108, 35})
	limitsPAL  = image.Rect(0, 0, 640, 576).Add(image.Point{128, 45})
)

func SetupNTSC() {
	fb := Framebuffer()
	SetFramebuffer(nil)

	regs.vIntr.Store(2)
	regs.burst.Store(62<<20 | 5<<16 | 34<<8 | 57)
	regs.vSync.Store(525)
	regs.hSync.Store(0b00000<<16 | 3093)
	regs.hSyncLeap.Store(3093<<16 | 3093)
	regs.vBurst.Store(14<<16 | 516)

	limits = limitsNTSC
	SetScale(limits)

	SetFramebuffer(fb)
}

func SetupPAL(interlace, pal60 bool) {
	Interlaced = interlace
	fb := Framebuffer()
	SetFramebuffer(nil)

	lines := uint32(625)
	limits = limitsPAL
	if pal60 {
		lines = 525
		limits.Min.Y = limitsNTSC.Min.Y
		limits.Max.Y = limitsNTSC.Max.Y
	}
	if Interlaced {
		lines -= 1
	}

	regs.vSync.Store(lines)
	regs.vIntr.Store(2)
	regs.burst.Store(64<<20 | 4<<16 | 35<<8 | 58)
	regs.hSync.Store(0b10101<<16 | 3177)
	regs.hSyncLeap.Store(3183<<16 | 3182)
	regs.vBurst.Store(9<<16 | 619)

	SetScale(limits)

	SetFramebuffer(fb)
}

func SetScale(r image.Rectangle) image.Rectangle {
	if r.Dx() > limits.Dx() {
		r.Max.X -= r.Dx() - limits.Dx()
	}
	if r.Dy() > limits.Dy() {
		r.Max.Y -= r.Dy() - limits.Dy()
	}

	var shift image.Point
	shift.X = max(limits.Min.X-r.Min.X, 0) + min(limits.Max.X-r.Max.X, 0)
	shift.Y = max(limits.Min.Y-r.Min.Y, 0) + min(limits.Max.Y-r.Max.Y, 0)
	r = r.Add(shift)

	regs.hVideo.Store(uint32(r.Min.X<<16 | r.Max.X))
	regs.vVideo.Store(uint32(r.Min.Y<<16 | r.Max.Y))

	rect = r
	return r
}

var rect image.Rectangle

// Sets the framebuffer to the specified texture and enables video output.  If a
// framebuffer was already set, does a fast switch without reconfigure.  Setting
// a nil framebuffer will disable video output.
func SetFramebuffer(fb texture.Texture) {
	if fb == nil {
		regs.control.Store(0)
		framebuffer = fb
	} else if framebuffer == nil ||
		framebuffer.BPP() != fb.BPP() ||
		framebuffer.Bounds().Size() != fb.Bounds().Size() {

		width, height := uint32(fb.Bounds().Dx()), uint32(fb.Bounds().Dy())
		control := uint32(bpp(fb.BPP())) | uint32(AANone)
		r := NativeResolution()
		stride := uint32(fb.Stride())

		if Interlaced {
			control |= uint32(ControlSerrate)
			r.Y = r.Y << 1
			stride = stride << 1
		}

		regs.xScale.Store((width<<10 + uint32(r.X>>1)) / uint32(r.X))
		regs.yScale.Store((height<<10 + uint32(r.Y>>2)) / uint32(r.Y>>1))
		regs.width.Store(stride)
		regs.origin.Store(cpu.PhysicalAddress(fb.Addr()))

		framebuffer = fb
		regs.control.Store(control)
	} else {
		addr := cpu.PhysicalAddress(fb.Addr())
		if Interlaced && Odd.Load() {
			addr += uint32(texture.PixelsToBytes(fb.Stride(), fb.BPP()))
		}
		regs.origin.Store(addr)
	}
}

// Returns the currently displayed framebuffer.
func Framebuffer() texture.Texture {
	return framebuffer
}

// Returns the currently configured native resolution.  Use this resolution for
// the framebuffer to avoid scaling, i.e. get pixel perfect output.
func NativeResolution() image.Point {
	h := regs.hVideo.Load()
	v := regs.vVideo.Load()
	return image.Point{
		int(h&0xffff - h>>16),
		int(v&0xffff - v>>16),
	}
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
