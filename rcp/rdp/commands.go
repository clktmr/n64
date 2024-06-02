// This file gives direct access to some of the low-level RDP commands, which
// can be used for simple 2D graphics.  For 3D graphics the GBI interface of the
// RSP should be used.  Further documentation can be found in the official docs.
package rdp

import (
	"fmt"
	"image"
	"image/color"
	"n64/debug"
	"n64/rcp/cpu"
	"unsafe"
)

// Each RDP command is a 64-bit dword, but needs to be stored as two words to
// get endianess right.
type Command struct{ UW, LW uint32 }

type DisplayList struct {
	RDPState

	beginState RDPState
	commands   []Command
}

type RDPState struct {
	otherModes ModeFlags
	fillColor  color.Color
	format     ImageFormat
	bbp        BitDepth
}

func NewDisplayList() *DisplayList {
	return &DisplayList{
		commands: make([]Command, 0, 32),
	}
}

var state RDPState

func Run(dl *DisplayList) {
	dl.sync(Full)

	cmds := dl.commands
	elemSize := unsafe.Sizeof(cmds[0])
	start := uintptr(unsafe.Pointer(&cmds[0]))
	end := uintptr(unsafe.Pointer(&cmds[len(cmds)-1])) + elemSize
	length := int(end - start)

	cpu.Writeback(start, length)

	debug.Assert(regs.status.LoadBits(startPending|endPending) == 0, "concurrent rdp access")

	regs.status.Store(clrFlush | clrFreeze | clrXbus) // TODO why? see libdragon

	FullSync.Clear()

	state = dl.RDPState

	regs.start.Store(uint32(cpu.PhysicalAddress(start)))
	regs.end.Store(uint32(cpu.PhysicalAddress(end)))

	FullSync.Sleep(-1)

	// TODO runtime.KeepAlive(cmds) until next call
}

type ImageFormat uint32

const (
	RGBA ImageFormat = iota << 21
	YUV
	ColorIdx // Color Palette
	IA       // Intensity with alpha
	I        // Intensity
)

type BitDepth uint32

const (
	BBP4 BitDepth = iota << 19
	BBP8
	BBP16
	BBP32
)

// Shift a number of pixels left by pixelsToBytes to get their size in bytes.
func pixelsToBytes(pixels int, bbp BitDepth) int {
	shift := int(bbp)>>19 - 1
	if shift < 0 {
		return pixels >> -shift
	}
	return pixels << shift
}

// Sets the framebuffer to render the final image into.
func (dl *DisplayList) SetColorImage(addr uintptr, width uint32, format ImageFormat, bbp BitDepth) {
	debug.Assert(addr%64 == 0, fmt.Sprintf("rdp framebuffer must be 64 byte aligned %x", addr))
	debug.Assert(width < 1<<9, "rdp framebuffer width too big")

	// TODO according to wiki, a sync *might* be needed in edge cases

	dl.commands = append(dl.commands, Command{
		UW: uint32((0xff << 24) | uint32(format) | uint32(bbp) | (width - 1)),
		LW: cpu.PhysicalAddress(addr),
	})

	dl.format = format
	dl.bbp = bbp
}

// Sets the zbuffer.  Width is taken from SetColorImage, bbp is always 18.
func (dl *DisplayList) SetDepthImage(addr uintptr) {
	debug.Assert(addr%64 == 0, "rdp zbuffer must be 64 byte aligned")

	dl.commands = append(dl.commands, Command{
		UW: 0xfe << 24,
		LW: cpu.PhysicalAddress(addr),
	})
}

// Sets the image where LoadTile and LoadBlock will copy their data from.
func (dl *DisplayList) SetTextureImage(addr uintptr, width uint32, bbp BitDepth) {
	debug.Assert(addr%8 != 0, "rdp texture must be 8 byte aligned")
	debug.Assert(width < 1<<9, "rdp texture width too big")

	dl.commands = append(dl.commands, Command{
		// according to wiki, format[23:21] has no effect
		UW: (0xfd << 24) | uint32(bbp) | (width - 1),
		LW: cpu.PhysicalAddress(addr),
	})
}

type TileDescFlags uint32

const (
	MirrorS TileDescFlags = 1 << 8
	ClampS  TileDescFlags = 1 << 9
	MirrorT TileDescFlags = 1 << 18
	ClampT  TileDescFlags = 1 << 19
)

type TileDescriptor struct {
	Format         ImageFormat
	Size           BitDepth
	Line           uint16 // 9 bit
	TMEMAddr       uint16 // 9 bit
	Idx            uint8  // 3 bit
	Palette        uint8  // 4 bit
	MaskT, MaskS   uint8  // 4 bit
	ShiftT, ShiftS uint8  // 4 bit
	Flags          TileDescFlags
}

// Sets a tile's properties.  There are a total of eight tiles, identified by
// the Idx field, which can later be referenced in other commands, e.g.
// LoadTile().
func (dl *DisplayList) SetTile(ts TileDescriptor) {
	cmdUW := 0xf5<<24 | uint32(ts.Format) | uint32(ts.Size)
	cmdUW |= uint32(ts.Line)<<9 | uint32(ts.TMEMAddr)

	cmdLW := uint32(ts.Idx)<<24 | uint32(ts.Palette)<<20
	cmdLW |= uint32(ts.MaskT)<<14 | uint32(ts.ShiftT)<<10
	cmdLW |= uint32(ts.MaskS)<<4 | uint32(ts.ShiftS)
	cmdLW |= uint32(ts.Flags)
	dl.commands = append(dl.commands, Command{
		UW: cmdUW,
		LW: cmdLW,
	})
}

// Copies a tile into TMEM.  The tile is copied from the texture image, which
// must be set prior via SetTextureImage().
func (dl *DisplayList) LoadTile(idx uint8, r image.Rectangle) {
	cmdUW := 0xf4<<24 | uint32(r.Min.X)<<14 | uint32(r.Min.Y)<<2
	cmdLW := uint32(idx)<<24 | uint32(r.Max.X)<<14 | uint32(r.Max.Y)<<2
	dl.commands = append(dl.commands, Command{
		UW: cmdUW,
		LW: cmdLW})
}

// Mode flags for the SetOtherModes() command.
// TODO Blend modewords (bits 16-31)
type ModeFlags uint64

const (
	AlphaCompare ModeFlags = 1 << iota
	DitherAlpha
	ZPerPrimitive // Use depth value from SetPrimitiveDepth instead per-pixel calculation
	AntiAlias
	ZCompare // Compare zbuffer
	ZUpdate  // Update zbuffer
	ImageRead
	ColorOnCoverage

	CvgTimesAlphaVG ModeFlags = 1 << (iota + 4)
	AlphaCvgSelect
	ForceBlend

	ChromaKeying ModeFlags = 1 << (iota + 29)
	ConvertOne
	BiLerp1
	BiLerp0
	MidTexel
	SampleType
	TLUTType
	TLUT
	TextureLOD
	TextureSharpen
	TextureDetail
	TexturePerpective
	AtomicPrimitive = 1 << 55
)

type CycleType uint64

const (
	CycleTypeOne CycleType = iota << 52
	CycleTypeTwo
	CycleTypeCopy
	CycleTypeFill
)

type RGBDither uint64

const (
	RGBDitherMagicSquare RGBDither = iota << 38
	RGBDitherBayer
	RGBDitherNoise
	RGBDitherNone
)

type AlphaDither uint64

const (
	AlphaDitherPattern AlphaDither = iota << 36
	AlphaDitherInvPattern
	AlphaDitherNoise
	AlphaDitherNone
)

type ZMode uint64

const (
	ZmodeOpaque ZMode = iota << 10
	ZmodeInterpenetrating
	ZmodeTransparent
	ZmodeDecal
)

type CvgDest uint64

const (
	CvgDestClamp CvgDest = iota << 8
	CvgDestWrap
	CvgDestZap
	CvgDestSave
)

func (dl *DisplayList) SetOtherModes(flags ModeFlags, ct CycleType, cDith RGBDither, aDith AlphaDither, zMode ZMode, cvgDest CvgDest) {
	debug.Assert(!(ct == CycleTypeCopy && dl.bbp == BBP32), "COPY mode unavailable for 32-bit framebuffer")
	debug.Assert(!(ct == CycleTypeFill && dl.bbp == BBP4), "FILL mode unavailable for 4-bit framebuffer")

	m := flags | ModeFlags(ct) | ModeFlags(cDith) | ModeFlags(aDith) | ModeFlags(zMode) | ModeFlags(cvgDest)

	if m == dl.otherModes {
		return // avoid costly pipeline sync
	}
	dl.otherModes = m

	dl.sync(Pipe)

	// TODO merge with previous command if also SetOtherModes

	cmd := 0xef00_000f_0000_0000 | m
	dl.commands = append(dl.commands, Command{
		UW: uint32(cmd >> 32),
		LW: uint32(cmd),
	})
}

type InterlaceFrame uint8

const (
	InterlaceNone InterlaceFrame = 0 // draw all lines
	InterlaceOdd  InterlaceFrame = 2 // skip odd lines
	InterlaceEven InterlaceFrame = 3 // skip even lines
)

// Everything outside `r` is skipped when rendering.  Additionally odd or even
// lines can be skipped to render interlaced frames.
func (dl *DisplayList) SetScissor(r image.Rectangle, i InterlaceFrame) {
	cmd := uint64(0xed << 56)
	cmd |= uint64(r.Min.X<<46) | uint64(r.Min.Y<<34) | uint64(r.Max.X<<14) | uint64(r.Max.Y<<2)
	cmd |= uint64(i) << 24
	dl.commands = append(dl.commands, Command{
		UW: uint32(cmd >> 32),
		LW: uint32(cmd),
	})
}

// Sets the color for subsequent FillRectangle() calls.
func (dl *DisplayList) SetFillColor(c color.Color) {
	if c == dl.fillColor {
		return // avoid costly pipeline sync
	}
	dl.fillColor = c // FIXME needs copy

	dl.sync(Pipe)

	r, g, b, a := c.RGBA()
	var ci uint32
	if dl.bbp == BBP32 {
		ci = ((r >> 8) << 24) | ((g >> 8) << 16) | ((b >> 8) << 8) | (a >> 8)
	} else if dl.bbp == BBP16 {
		ci = ((r >> 11) << 11) | ((g >> 11) << 6) | ((b >> 11) << 1) | (a >> 15)
		ci |= ci << 16
	} else if dl.bbp == BBP8 {
		ci = ((a >> 8) << 24) | ((a >> 8) << 16) | ((a >> 8) << 8) | (a >> 8)
	} else {
		debug.Assert(false, "fill color unavailable for 4-bit framebuffer")
	}
	dl.commands = append(dl.commands, Command{
		UW: 0xf700_0000,
		LW: ci,
	})
}

// Draws a rectangle filled with the color set by SetFillColor().
func (dl *DisplayList) FillRectangle(r image.Rectangle) {
	cmd := uint64(0xf6 << 56)
	cmd |= uint64(r.Max.X<<46) | uint64(r.Max.Y<<34) | uint64(r.Min.X<<14) | uint64(r.Min.Y<<2)
	dl.commands = append(dl.commands, Command{
		UW: uint32(cmd >> 32),
		LW: uint32(cmd),
	})
}

// Draws a textured rectangle.
func (dl *DisplayList) TextureRectangle(r image.Rectangle, tileIdx uint8) {
	cmdUW := 0xe4<<24 | uint32(r.Max.X)<<14 | uint32(r.Max.Y)<<2
	cmdLW := uint32(tileIdx)<<24 | uint32(r.Min.X)<<14 | uint32(r.Min.Y)<<2
	dl.commands = append(dl.commands, []Command{
		Command{UW: cmdUW, LW: cmdLW},
		Command{UW: uint32(0), LW: uint32(1<<28 | 1<<10)},
	}...)
}

type SyncCommand uint32

const (
	// Waits until all previous commands have finished reading and writing
	// to RDRAM.  Additionally raises the RDP interrupt.  Use to sync memory
	// access between RDP and other components (e.g. switching framebuffers)
	// or when changing RDPs RDRAM buffers (e.g. Render to texture).
	Full SyncCommand = 0xe900_0000

	// Stalls pipeline for exactly 25 GCLK cycles.  Guarantees loading
	// pipeline is safe for use.
	Load SyncCommand = 0xf100_0000

	// Stalls pipeline for exactly 50 GCLK cycles.  Guarantees any
	// preceeding primitives have finished rendering and it's safe to change
	// rendering modes.
	Pipe SyncCommand = 0xe700_0000

	// Stalls pipeline for exactly 33 GCLK cycles.  Guarantees that any
	// preceding primitives have finished using tile information and
	// it's safe to modify tile descriptors.
	Tile SyncCommand = 0xe800_0000
)

func (dl *DisplayList) sync(s SyncCommand) {
	last := SyncCommand(dl.commands[len(dl.commands)-1].UW)
	if s == last {
		return
	}

	dl.commands = append(dl.commands, Command{
		UW: uint32(s),
		LW: 0x0,
	})
}
