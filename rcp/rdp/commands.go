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

// Each RDP command is a 64-bit qword, but needs to be stored as two words to
// get endianess right.
type Command struct{ UW, LW uint32 }

type DisplayList struct {
	RDPState

	beginState RDPState
	commands   []Command
}

type RDPState struct {
	combineMode      CombineMode
	otherModes       ModeFlags
	fillColor        color.RGBA
	blendColor       color.RGBA
	primitiveColor   color.RGBA
	environmentColor color.RGBA
	format           ImageFormat
	bbp              BitDepth
}

const DisplayListLength = 256

func NewDisplayList() *DisplayList {
	return &DisplayList{
		commands: cpu.MakePaddedSlice[Command](DisplayListLength)[:0],
	}
}

var state RDPState

func Run(dl *DisplayList) {
	dl.sync(Full)

	cmds := dl.commands
	elemSize := unsafe.Sizeof(cmds[0])
	start := uintptr(unsafe.Pointer(&cmds[0]))
	end := uintptr(unsafe.Pointer(&cmds[len(cmds)-1])) + elemSize

	cpu.WritebackSlice(dl.commands)

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

func (dl *DisplayList) push(cmd Command) {
	dl.commands = dl.commands[:len(dl.commands)+1]
	dl.commands[len(dl.commands)-1] = cmd
}

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

	dl.push(Command{
		UW: uint32((0xff << 24) | uint32(format) | uint32(bbp) | (width - 1)),
		LW: cpu.PhysicalAddress(addr),
	})

	dl.format = format
	dl.bbp = bbp
}

// Sets the zbuffer.  Width is taken from SetColorImage, bbp is always 18.
func (dl *DisplayList) SetDepthImage(addr uintptr) {
	debug.Assert(addr%64 == 0, "rdp zbuffer must be 64 byte aligned")

	dl.push(Command{
		UW: 0xfe << 24,
		LW: cpu.PhysicalAddress(addr),
	})
}

// Sets the image where LoadTile and LoadBlock will copy their data from.
func (dl *DisplayList) SetTextureImage(addr uintptr, width uint32, bbp BitDepth) {
	debug.Assert(addr%8 == 0, "rdp texture must be 8 byte aligned")
	debug.Assert(width < 1<<9, "rdp texture width too big")

	dl.push(Command{
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

var supportedFormat = map[ImageFormat]map[BitDepth]bool{
	RGBA:     {BBP16: true, BBP32: true},
	YUV:      {BBP16: true},
	ColorIdx: {BBP4: true, BBP8: true},
	IA:       {BBP4: true, BBP8: true, BBP16: true},
	I:        {BBP4: true, BBP8: true},
}

type TileDescriptor struct {
	Format         ImageFormat
	Size           BitDepth
	Line           uint16 // 9 bit; line length in qwords
	Addr           uint16 // 9 bit; TMEM address in qwords
	Idx            uint8  // 3 bit; Tile index
	Palette        uint8  // 4 bit; Palette index
	MaskT, MaskS   uint8  // 4 bit
	ShiftT, ShiftS uint8  // 4 bit
	Flags          TileDescFlags
}

// Sets a tile's properties.  There are a total of eight tiles, identified by
// the Idx field, which can later be referenced in other commands, e.g.
// LoadTile().
func (dl *DisplayList) SetTile(ts TileDescriptor) {
	debug.Assert(ts.Line < 1<<9, "tile line length out of bounds")
	debug.Assert(ts.Addr < 1<<9, "tile addr out of bounds")
	debug.Assert(ts.Idx < 1<<3, "tile index out of bounds")
	debug.Assert(ts.Palette < 1<<4, "tile palette index out of bounds")
	debug.Assert(ts.MaskT < 1<<4, "tile mask out of bounds")
	debug.Assert(ts.MaskS < 1<<4, "tile mask out of bounds")
	debug.Assert(ts.ShiftT < 1<<4, "tile shift out of bounds")
	debug.Assert(ts.ShiftS < 1<<4, "tile shift out of bounds")
	debug.Assert(supportedFormat[ts.Format][ts.Size], fmt.Sprintf("tile unsupported format: %x %x", ts.Format, ts.Size))

	// some formats must indicate 16 byte instead of 8 byte texels
	if ts.Size == BBP32 && (ts.Format == RGBA || ts.Format == YUV) {
		ts.Line = ts.Line >> 1
	}

	cmdUW := 0xf5<<24 | uint32(ts.Format) | uint32(ts.Size)
	cmdUW |= uint32(ts.Line)<<9 | uint32(ts.Addr)

	cmdLW := uint32(ts.Idx)<<24 | uint32(ts.Palette)<<20
	cmdLW |= uint32(ts.MaskT)<<14 | uint32(ts.ShiftT)<<10
	cmdLW |= uint32(ts.MaskS)<<4 | uint32(ts.ShiftS)
	cmdLW |= uint32(ts.Flags)
	dl.push(Command{
		UW: cmdUW,
		LW: cmdLW,
	})
}

// Copies a tile into TMEM.  The tile is copied from the texture image, which
// must be set prior via SetTextureImage().
func (dl *DisplayList) LoadTile(idx uint8, r image.Rectangle) {
	dl.sync(Load)

	cmdUW := 0xf4<<24 | uint32(r.Min.X)<<14 | uint32(r.Min.Y)<<2
	cmdLW := uint32(idx)<<24 | uint32(r.Max.X-1)<<14 | uint32(r.Max.Y-1)<<2

	dl.push(Command{UW: cmdUW, LW: cmdLW})
}

// Tile size is automatically set on LoadTile(), but can be overidden with
// SetTileSize().
func (dl *DisplayList) SetTileSize(idx uint8, r image.Rectangle) {
	cmdUW := 0xf2<<24 | uint32(r.Min.X)<<14 | uint32(r.Min.Y)<<2
	cmdLW := uint32(idx)<<24 | uint32(r.Max.X-1)<<14 | uint32(r.Max.Y-1)<<2

	dl.push(Command{UW: cmdUW, LW: cmdLW})
}

// Mode flags for the SetOtherModes() command.
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

type BlenderPM uint64

const (
	BlenderPMColorCombiner BlenderPM = iota
	BlenderPMFramebuffer
	BlenderPMBlendColor
	BlenderPMFogColor
)

type BlenderA uint64

const (
	BlenderAColorCombinerAlpha BlenderA = iota
	BlenderAFogAlpha
	BlenderAShadeAlpha
	BlenderAZero
)

type BlenderB uint64

const (
	BlenderBOneMinusAlphaA BlenderB = iota
	BlenderBFramebufferCvg
	BlenderBOne
	BlenderBZero
)

type BlendMode struct {
	P1, P2 BlenderPM
	M1, M2 BlenderPM
	A1, A2 BlenderA
	B1, B2 BlenderB
}

func (c *BlendMode) modeFlags() ModeFlags {
	return (ModeFlags(c.B2<<16) | ModeFlags(c.B1<<18) |
		ModeFlags(c.M2<<20) | ModeFlags(c.M1<<22) |
		ModeFlags(c.A2<<24) | ModeFlags(c.A1<<26) |
		ModeFlags(c.P2<<28) | ModeFlags(c.P1<<30))
}

func (dl *DisplayList) SetOtherModes(
	flags ModeFlags,
	ct CycleType,
	cDith RGBDither,
	aDith AlphaDither,
	zMode ZMode,
	cvgDest CvgDest,
	blend BlendMode,
) {
	debug.Assert(!(ct == CycleTypeCopy && dl.bbp == BBP32), "COPY mode unavailable for 32-bit framebuffer")
	debug.Assert(!(ct == CycleTypeFill && dl.bbp == BBP4), "FILL mode unavailable for 4-bit framebuffer")

	m := flags | blend.modeFlags()
	m |= ModeFlags(ct) | ModeFlags(cDith) | ModeFlags(aDith) | ModeFlags(zMode) | ModeFlags(cvgDest)

	if m == dl.otherModes {
		return
	}
	dl.otherModes = m

	dl.sync(Pipe)

	// TODO merge with previous command if also SetOtherModes

	cmd := 0xef00_000f_0000_0000 | m
	dl.push(Command{
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
	if dl.otherModes&ModeFlags(CycleTypeCopy|CycleTypeFill) != 0 {
		r.Max = r.Max.Sub(image.Point{1, 1})
	}

	cmd := uint64(0xed << 56)
	cmd |= uint64(r.Min.X<<46) | uint64(r.Min.Y<<34) | uint64(r.Max.X<<14) | uint64(r.Max.Y<<3)
	cmd |= uint64(i) << 24
	dl.push(Command{
		UW: uint32(cmd >> 32),
		LW: uint32(cmd),
	})
}

// Sets the color for subsequent FillRectangle() calls.
func (dl *DisplayList) SetFillColor(c color.Color) {
	cRGBA := color.RGBAModel.Convert(c).(color.RGBA)
	if c == dl.fillColor {
		return
	}
	dl.fillColor = cRGBA

	dl.sync(Pipe)

	r, g, b, a := dl.fillColor.RGBA()
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
	dl.push(Command{
		UW: 0xf700_0000,
		LW: ci,
	})
}

func (dl *DisplayList) SetBlendColor(c color.Color) {
	cRGBA := color.RGBAModel.Convert(c).(color.RGBA)
	if cRGBA == dl.blendColor {
		return
	}
	dl.blendColor = cRGBA

	dl.sync(Pipe)

	dl.push(Command{
		UW: 0xf900_0000,
		LW: uint32(dl.blendColor.R)<<24 | uint32(dl.blendColor.G)<<16 |
			uint32(dl.blendColor.B)<<8 | uint32(dl.blendColor.A),
	})
}

func (dl *DisplayList) SetPrimitiveColor(c color.Color) {
	cRGBA := color.RGBAModel.Convert(c).(color.RGBA)
	if cRGBA == dl.primitiveColor {
		return
	}
	dl.primitiveColor = cRGBA

	dl.sync(Pipe)

	dl.push(Command{
		UW: 0xfa00_0000,
		LW: uint32(dl.primitiveColor.R)<<24 | uint32(dl.primitiveColor.G)<<16 |
			uint32(dl.primitiveColor.B)<<8 | uint32(dl.primitiveColor.A),
	})
}

func (dl *DisplayList) SetEnvironmentColor(c color.Color) {
	cRGBA := color.RGBAModel.Convert(c).(color.RGBA)
	if cRGBA == dl.environmentColor {
		return
	}
	dl.environmentColor = cRGBA

	dl.sync(Pipe)

	dl.push(Command{
		UW: 0xfb00_0000,
		LW: uint32(dl.environmentColor.R)<<24 | uint32(dl.environmentColor.G)<<16 |
			uint32(dl.environmentColor.B)<<8 | uint32(dl.environmentColor.A),
	})
}

type CombineSource uint32

const (
	CombineCombined CombineSource = iota
	CombineTex0
	CombineTex1
	CombinePrimitive
	CombineShade
	CombineEnvironment
)

type CombineParams struct{ A, B, C, D CombineSource }
type CombinePass struct{ RGB, Alpha CombineParams }
type CombineMode struct{ One, Two CombinePass }

func (dl *DisplayList) SetCombineMode(m CombineMode) {
	if dl.combineMode == m {
		return
	}
	dl.combineMode = m

	dl.sync(Pipe)

	dl.push(Command{
		UW: uint32(0xfc00_0000 |
			m.One.RGB.A<<20 | m.One.RGB.C<<15 |
			m.One.Alpha.A<<12 | m.One.Alpha.C<<9 |
			m.Two.RGB.A<<5 | m.Two.RGB.C),
		LW: uint32(0x0 |
			m.One.RGB.B<<28 | m.Two.RGB.B<<24 |
			m.Two.Alpha.A<<21 | m.Two.Alpha.C<<18 |
			m.One.RGB.D<<15 | m.One.Alpha.B<<12 | m.One.Alpha.D<<9 |
			m.Two.RGB.D<<6 | m.Two.Alpha.B<<3 | m.Two.Alpha.D,
		),
	})
}

// Draws a rectangle filled with the color set by SetFillColor().
func (dl *DisplayList) FillRectangle(r image.Rectangle) {
	if dl.otherModes&ModeFlags(CycleTypeCopy|CycleTypeFill) != 0 {
		r.Max = r.Max.Sub(image.Point{1, 1})
	}

	cmd := uint64(0xf6 << 56)
	cmd |= uint64(r.Max.X<<46) | uint64(r.Max.Y<<34) | uint64(r.Min.X<<14) | uint64(r.Min.Y<<2)
	dl.push(Command{
		UW: uint32(cmd >> 32),
		LW: uint32(cmd),
	})
}

// Draws a textured rectangle.
func (dl *DisplayList) TextureRectangle(r image.Rectangle, p image.Point, scale image.Point, tileIdx uint8) {
	if dl.otherModes&ModeFlags(CycleTypeCopy|CycleTypeFill) != 0 {
		r.Max = r.Max.Sub(image.Point{1, 1})
	}

	cmdUW := 0xe4<<24 | uint32(r.Max.X)<<14 | uint32(r.Max.Y)<<2
	cmdLW := uint32(tileIdx)<<24 | uint32(r.Min.X)<<14 | uint32(r.Min.Y)<<2
	dl.push(Command{UW: cmdUW, LW: cmdLW})
	dl.push(Command{
		UW: uint32(p.X<<21 | p.Y<<5),
		LW: uint32(((0x8000/scale.X)>>5)<<16 | (0x8000/scale.Y)>>5),
	})
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

	dl.push(Command{UW: uint32(s), LW: 0x0})
}
