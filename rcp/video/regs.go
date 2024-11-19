// Video DAC which reads an image from RDRAM and outputs it to screen as either
// NTSC, PAL or M-PAL.  This package ensures register writes are done only
// during vblank.
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
	// True if current configuration is interlaced.
	// There are three parts involved in generating interlaced video output:
	// (1) Framebuffer sampling:  This must be done manually by shifting the
	// framebuffer by one line.  If the framebuffer has the same vertical
	// resolution as video lines, it must be shifted by one pixel.  This can
	// be done by offsetting the origin register by one stride.  At lower
	// resolutions the yScale's subpixel offset must be used.
	// (2) Interrupt handling:  Setting vSync register's bit 0 causes
	// vCurrent to report alternating fields.  This information can be used
	// to select the correct field when shifting in (1).
	// (3) Video signal generation:  Interlaced video output is enabled by
	// ControlSerrate, which "serrates" the vertical sync pulse, shifting
	// even and odd fields by one halfline to another, effectively doubling
	// vertical resolution at the cost of flickering.  It also reduces the
	// scanlines.
	interlaced bool

	VSync bool = true

	limits     image.Rectangle
	limitsNTSC = image.Rect(0, 0, 640, 480).Add(image.Point{108, 35})
	limitsPAL  = image.Rect(0, 0, 640, 576).Add(image.Point{128, 45})
)

func SetupNTSC(interlace bool) {
	fb := Framebuffer()
	SetFramebuffer(nil)

	interlaced = interlace
	lines := uint32(525)
	if interlaced {
		lines -= 1
	}

	regs.vSync.Store(lines)
	regs.hSync.Store(0b00000<<16 | 3093)
	regs.hSyncLeap.Store(3093<<16 | 3093)
	regs.vBurst.Store(14<<16 | 516)
	regs.burst.Store(62<<20 | 5<<16 | 34<<8 | 57)

	limits = limitsNTSC
	SetScale(limits)

	regs.vIntr.Store(2)
	SetFramebuffer(fb)
}

func SetupPAL(interlace, pal60 bool) {
	fb := Framebuffer()
	SetFramebuffer(nil)

	interlaced = interlace
	lines := uint32(625)
	limits = limitsPAL
	if pal60 {
		lines = 525
		limits.Min.Y = limitsNTSC.Min.Y
		limits.Max.Y = limitsNTSC.Max.Y
	}
	if interlaced {
		lines -= 1
	}

	regs.vSync.Store(lines)
	regs.hSync.Store(0b10101<<16 | 3177)
	regs.hSyncLeap.Store(3183<<16 | 3182)
	regs.vBurst.Store(9<<16 | 619)
	regs.burst.Store(64<<20 | 4<<16 | 35<<8 | 58)

	SetScale(limits)

	regs.vIntr.Store(2)
	SetFramebuffer(fb)
}

// Scale returns the rectangle inside the current video standards boundaries
// which contains the video output.
func Scale() image.Rectangle {
	return scale.Get()
}

// SetScale sets and returns the rectangle which contains the video output.  If
// r's dimensions exceed the limits of the screen it will be shrunk, possibly
// changing aspect ratio.  If r's offset exceeds the screen's limits, it will be
// shifted accordingly.  The new scale will be applied during the next vblank.
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

	scale.Store(r)
	return r
}

// Sets the framebuffer to the specified texture and enables video output.  If a
// framebuffer was already set, does a fast switch without reconfigure.  Setting
// a nil framebuffer will disable video output.
func SetFramebuffer(fb texture.Texture) {
	currentFb := framebuffer.Get()
	if fb == nil {
		regs.control.Store(0)
		framebuffer.Store(nil)
		framebuffer.Store(nil) // make sure no reference is hold
	} else if currentFb == nil ||
		currentFb.BPP() != fb.BPP() ||
		currentFb.Bounds().Size() != fb.Bounds().Size() {

		control := uint32(bpp(fb.BPP())) | uint32(AAResampling)
		if interlaced {
			control |= uint32(ControlSerrate)
		}

		fbSize := fb.Bounds().Size()
		videoSize := Scale().Size()
		regs.xScale.Store(uint32((fbSize.X<<10 + videoSize.X>>1) / (videoSize.X)))
		regs.yScale.Store(uint32((fbSize.Y<<10 + videoSize.Y>>2) / (videoSize.Y >> 1)))
		regs.width.Store(uint32(fb.Stride()))

		framebuffer.Store(fb)
		updateFramebuffer(fb)
		regs.control.Store(control)
	} else {
		framebuffer.Store(fb)
		if !VSync {
			updateFramebuffer(fb)
		}
	}
}

// Returns the currently displayed framebuffer.
func Framebuffer() texture.Texture {
	return framebuffer.Get()
}

// Returns the currently configured native resolution.  Use this resolution or a
// divisible of it for the framebuffer to avoid scaling artifacts, i.e. get
// pixel perfect output.  Note that the resolution might have non-square pixels.
func NativeResolution() image.Point {
	resolution := Scale().Size()
	if !interlaced {
		resolution.Y = resolution.Y >> 1
	}
	return resolution
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
