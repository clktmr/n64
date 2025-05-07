// Package video provides configuration of the analog video output.
//
// The video DAC reads a framebuffer image from RDRAM and outputs it to screen
// as either NTSC, PAL or M-PAL. All function are safe to call at any time, as
// this implementation ensures reconfiguration is done only during vblank.
package video

import (
	"embedded/mmio"
	"image"
	"unsafe"

	"github.com/clktmr/n64/debug"
	"github.com/clktmr/n64/machine"
	"github.com/clktmr/n64/rcp/cpu"
	"github.com/clktmr/n64/rcp/texture"
)

var regs *registers = (*registers)(unsafe.Pointer(baseAddr))

const baseAddr uintptr = cpu.KSEG1 | 0x0440_0000

type registers struct {
	control   mmio.U32
	origin    mmio.R32[cpu.Addr]
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

type antiAliasMode uint32

const (
	aaNone       antiAliasMode = 3 << 8
	aaResampling antiAliasMode = 2 << 8
	aaEnabled    antiAliasMode = 1 << 8
	aaDedither   antiAliasMode = 0 << 8
)

type controlFlags uint32

const (
	controlDedither    controlFlags = 1 << 16
	controlSerrate     controlFlags = 1 << 6
	controlDivot       controlFlags = 1 << 4
	controlGamma       controlFlags = 1 << 3
	controlGammaDither controlFlags = 1 << 2
)

var (
	// True if current configuration is interlaced.
	// There are three parts involved in generating interlaced video output:
	// (1) Framebuffer sampling: This must be done manually by shifting the
	// framebuffer by one line. If the framebuffer has the same vertical
	// resolution as video lines, it must be shifted by one pixel. This can
	// be done by offsetting the origin register by one stride. At lower
	// resolutions the yScale's subpixel offset must be used.
	// (2) Interrupt handling: Setting vSync register's bit 0 causes
	// vCurrent to report alternating fields. This information can be used
	// to select the correct field when shifting in (1).
	// (3) Video signal generation: Interlaced video output is enabled by
	// ControlSerrate, which "serrates" the vertical sync pulse, shifting
	// even and odd fields by one halfline to another, effectively doubling
	// vertical resolution at the cost of flickering. It also reduces the
	// scanlines.
	interlaced bool

	limits     image.Rectangle
	limitsNTSC = image.Rect(0, 0, 640, 480).Add(image.Point{108, 35})
	limitsPAL  = image.Rect(0, 0, 640, 576).Add(image.Point{128, 45})
)

// Automatically configure video output based on detected console type.
func Setup(interlace bool) {
	switch machine.VideoType {
	case machine.VideoPAL:
		SetupPAL(interlace, false)
	case machine.VideoNTSC:
		SetupNTSC(interlace)

	}
}

// SetupNTSC configures video output to be NTSC. Can be used to force NTSC
// output on a PAL console.
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

// SetupPAL configures video output to be PAL. Can be used to force PAL output
// on a NTSC console.
//
// Additionally it allows to enable PAL60, which sets the same dimensions and
// refresh rate as NTSC. Since the pixel aspect ratio can't be changed this will
// result in black borders at top and bottom.
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
	s, _ := scale.Read()
	return s
}

// SetScale sets and returns the rectangle which contains the video output. If
// r's dimensions exceed the limits of the screen it will be shrunk, possibly
// changing aspect ratio. If r's dimensions result in exceeding scaling limits
// it will be enlarged, possibly changing aspect ratio. If r's offset exceeds
// the screen's limits, it will be shifted accordingly. The new scale will be
// applied during the next vblank.
func SetScale(r image.Rectangle) image.Rectangle {
	r = r.Canon()
	if r.Dx() > limits.Dx() {
		r.Max.X -= r.Dx() - limits.Dx()
	}
	if r.Dy() > limits.Dy() {
		r.Max.Y -= r.Dy() - limits.Dy()
	}

	minSize := limits.Size()
	if fb := Framebuffer(); fb != nil {
		minSize = fb.Bounds().Size()
	}
	minSize = minSize.Div(4).Mul(3) // best guess
	if r.Dx() < minSize.X {
		r.Max.X += minSize.X - r.Dx()
	}
	if r.Dy() < minSize.Y {
		r.Max.Y += minSize.Y - r.Dy()
	}

	var shift image.Point
	shift.X = max(limits.Min.X-r.Min.X, 0) + min(limits.Max.X-r.Max.X, 0)
	shift.Y = max(limits.Min.Y-r.Min.Y, 0) + min(limits.Max.Y-r.Max.Y, 0)
	r = r.Add(shift)

	scale.Put(r)
	return r
}

// VSync controls whether [SetFramebuffer] will wait for the next vblank to
// avoid tearing.
var VSync bool = true

// Sets the framebuffer to the specified texture and enables video output. If a
// framebuffer was already set, does a fast swap without reconfigure. Setting a
// nil framebuffer will disable video output.
//
// The framebuffer texture must be of type [texture.RGBA16] or [texture.RGBA32].
//
// If [VSync] is set, the SetFramebuffer returns immediately but the last
// framebuffer will still be in use until next vblank.
func SetFramebuffer(fb texture.Texture) {
	currentFb, _ := framebuffer.Read()
	if fb == nil {
		regs.control.Store(0)
		framebuffer.Put(nil)
	} else if currentFb == nil ||
		currentFb.BPP() != fb.BPP() ||
		currentFb.Bounds().Size() != fb.Bounds().Size() {

		control := uint32(bpp(fb.BPP())) | uint32(aaResampling)
		if interlaced {
			control |= uint32(controlSerrate)
		}

		regs.control.Store(0)

		framebuffer.Put(fb)
		fbSize := fb.Bounds().Size()
		videoSize := SetScale(Scale()).Size()
		regs.xScale.Store(uint32((fbSize.X<<10 + videoSize.X>>1) / (videoSize.X)))
		regs.yScale.Store(uint32((fbSize.Y<<10 + videoSize.Y>>2) / (videoSize.Y >> 1)))
		regs.width.Store(uint32(fb.Stride()))

		updateFramebuffer(fb)
		regs.control.Store(control)
	} else {
		framebuffer.Put(fb)
		if !VSync {
			updateFramebuffer(fb)
		}
	}
}

// Returns the currently displayed framebuffer.
func Framebuffer() texture.Texture {
	f, _ := framebuffer.Read()
	return f
}

// NativeResolution reports the currently configured resolution which is output
// to the screen. Use this resolution or a divisible of it for the framebuffer
// to avoid resampling by the video DAC, i.e. get pixel perfect output.
//
// Note that if interlacing is disabled the vertical resolution is cut in half,
// doubling aspect ratio.
func NativeResolution() image.Point {
	resolution := Scale().Size()
	if !interlaced {
		resolution.Y = resolution.Y >> 1
	}
	return resolution
}

func bpp(bpp texture.BitDepth) ColorDepth {
	switch bpp {
	case texture.BPP16:
		return BPP16
	case texture.BPP32:
		return BPP32
	default:
		debug.Assert(false, "video: unsupported framebuffer format")
	}
	return 0
}
