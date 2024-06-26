// This file gives direct access to some of the low-level RDP commands, which
// can be used for simple 2D graphics.  For 3D graphics the GBI interface of the
// RSP should be used.  Further documentation can be found in the official docs.
package rdp

import (
	"fmt"
	"image"
	"image/color"
	"unsafe"

	"github.com/clktmr/n64/debug"
	"github.com/clktmr/n64/rcp/cpu"
)

type Command uint64

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

type ImageFormat uint64

const (
	RGBA ImageFormat = iota << 53
	YUV
	ColorIdx // Color Palette
	IA       // Intensity with alpha
	I        // Intensity
)

type BitDepth uint64

const (
	BBP4 BitDepth = iota << 51
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
	shift := int(bbp)>>51 - 1
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

	dl.push(((0xff << 56) | Command(format) | Command(bbp) | Command(width-1)<<32) |
		Command(cpu.PhysicalAddress(addr)))

	dl.format = format
	dl.bbp = bbp
}

// Sets the zbuffer.  Width is taken from SetColorImage, bbp is always 18.
func (dl *DisplayList) SetDepthImage(addr uintptr) {
	debug.Assert(addr%64 == 0, "rdp zbuffer must be 64 byte aligned")

	dl.push(Command((0xfe << 56) | Command(cpu.PhysicalAddress(addr))))
}

// Sets the image where LoadTile and LoadBlock will copy their data from.
func (dl *DisplayList) SetTextureImage(addr uintptr, width uint32, bbp BitDepth) {
	debug.Assert(addr%8 == 0, "rdp texture must be 8 byte aligned")
	debug.Assert(width < 1<<9, "rdp texture width too big")

	// according to wiki, format[23:21] has no effect
	dl.push((0xfd << 56) | Command(bbp) | Command(width-1)<<32 |
		Command(cpu.PhysicalAddress(addr)))
}

type TileDescFlags uint64

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

	cmd := Command(0xf5<<56) | Command(ts.Format) | Command(ts.Size)
	cmd |= Command(ts.Line)<<41 | Command(ts.Addr)<<32
	cmd |= Command(ts.Idx)<<24 | Command(ts.Palette)<<20
	cmd |= Command(ts.MaskT)<<14 | Command(ts.ShiftT)<<10
	cmd |= Command(ts.MaskS)<<4 | Command(ts.ShiftS)
	cmd |= Command(ts.Flags)

	dl.push(cmd)
}

// Copies a tile into TMEM.  The tile is copied from the texture image, which
// must be set prior via SetTextureImage().
func (dl *DisplayList) LoadTile(idx uint8, r image.Rectangle) {
	dl.sync(Load)

	cmd := 0xf4<<56 | Command(r.Min.X)<<46 | Command(r.Min.Y)<<34
	cmd |= Command(idx)<<24 | Command(r.Max.X-1)<<14 | Command(r.Max.Y-1)<<2

	dl.push(cmd)
}

// Tile size is automatically set on LoadTile(), but can be overidden with
// SetTileSize().
func (dl *DisplayList) SetTileSize(idx uint8, r image.Rectangle) {
	cmd := 0xf2<<56 | Command(r.Min.X)<<46 | Command(r.Min.Y)<<34
	cmd |= Command(idx)<<24 | Command(r.Max.X-1)<<14 | Command(r.Max.Y-1)<<2

	dl.push(cmd)
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
	dl.push(Command(cmd))
}

type InterlaceFrame uint64

const (
	InterlaceNone InterlaceFrame = iota << 24 // draw all lines
	_
	InterlaceOdd  // skip odd lines
	InterlaceEven // skip even lines
)

// Everything outside `r` is skipped when rendering.  Additionally odd or even
// lines can be skipped to render interlaced frames.
func (dl *DisplayList) SetScissor(r image.Rectangle, il InterlaceFrame) {
	if dl.otherModes&ModeFlags(CycleTypeCopy|CycleTypeFill) != 0 {
		r.Max = r.Max.Sub(image.Point{1, 1})
	}

	cmd := 0xed<<56 | Command(il)
	cmd |= Command(r.Min.X<<46) | Command(r.Min.Y<<34) | Command(r.Max.X<<14) | Command(r.Max.Y<<3)

	dl.push(Command(cmd))
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
	dl.push(Command(0xf7<<56) | Command(ci))
}

func (dl *DisplayList) SetBlendColor(c color.Color) {
	cRGBA := color.RGBAModel.Convert(c).(color.RGBA)
	if cRGBA == dl.blendColor {
		return
	}
	dl.blendColor = cRGBA

	dl.sync(Pipe)

	dl.push(0xf9<<56 |
		Command(dl.blendColor.R)<<24 | Command(dl.blendColor.G)<<16 |
		Command(dl.blendColor.B)<<8 | Command(dl.blendColor.A))
}

func (dl *DisplayList) SetPrimitiveColor(c color.Color) {
	cRGBA := color.RGBAModel.Convert(c).(color.RGBA)
	if cRGBA == dl.primitiveColor {
		return
	}
	dl.primitiveColor = cRGBA

	dl.push(0xfa<<56 |
		Command(dl.primitiveColor.R)<<24 | Command(dl.primitiveColor.G)<<16 |
		Command(dl.primitiveColor.B)<<8 | Command(dl.primitiveColor.A))
}

func (dl *DisplayList) SetEnvironmentColor(c color.Color) {
	cRGBA := color.RGBAModel.Convert(c).(color.RGBA)
	if cRGBA == dl.environmentColor {
		return
	}
	dl.environmentColor = cRGBA

	dl.sync(Pipe)

	dl.push(0xfb<<56 |
		Command(dl.environmentColor.R)<<24 | Command(dl.environmentColor.G)<<16 |
		Command(dl.environmentColor.B)<<8 | Command(dl.environmentColor.A))
}

func (dl *DisplayList) SetCombineMode(m CombineMode) {
	if dl.combineMode == m {
		return
	}
	dl.combineMode = m

	dl.sync(Pipe)

	cmd := Command(0xfc<<56 |
		m.One.RGB.A<<52 | m.One.RGB.C<<47 |
		m.One.Alpha.A<<44 | m.One.Alpha.C<<41 |
		m.Two.RGB.A<<37 | m.Two.RGB.C<<32)
	cmd |= Command(0x0 |
		m.One.RGB.B<<28 | m.Two.RGB.B<<24 |
		m.Two.Alpha.A<<21 | m.Two.Alpha.C<<18 |
		m.One.RGB.D<<15 | m.One.Alpha.B<<12 | m.One.Alpha.D<<9 |
		m.Two.RGB.D<<6 | m.Two.Alpha.B<<3 | m.Two.Alpha.D)

	dl.push(cmd)
}

// Draws a rectangle filled with the color set by SetFillColor().
func (dl *DisplayList) FillRectangle(r image.Rectangle) {
	if dl.otherModes&ModeFlags(CycleTypeCopy|CycleTypeFill) != 0 {
		r.Max = r.Max.Sub(image.Point{1, 1})
	}

	cmd := 0xf6<<56 | Command(r.Max.X<<46) | Command(r.Max.Y<<34)
	cmd |= Command(r.Min.X<<14) | Command(r.Min.Y<<2)
	dl.push(cmd)
}

// Draws a textured rectangle.
func (dl *DisplayList) TextureRectangle(r image.Rectangle, p image.Point, scale image.Point, tileIdx uint8) {
	if dl.otherModes&ModeFlags(CycleTypeCopy|CycleTypeFill) != 0 {
		r.Max = r.Max.Sub(image.Point{1, 1})
	}

	cmd := 0xe4<<56 | Command(r.Max.X)<<46 | Command(r.Max.Y)<<34
	cmd |= Command(tileIdx)<<24 | Command(r.Min.X)<<14 | Command(r.Min.Y)<<2
	dl.push(cmd)
	dl.push(Command(p.X<<53) | Command(p.Y<<37) |
		Command(((0x8000/scale.X)>>5)<<16|(0x8000/scale.Y)>>5))
}

type SyncCommand uint64

const (
	// Waits until all previous commands have finished reading and writing
	// to RDRAM.  Additionally raises the RDP interrupt.  Use to sync memory
	// access between RDP and other components (e.g. switching framebuffers)
	// or when changing RDPs RDRAM buffers (e.g. Render to texture).
	Full SyncCommand = 0xe9 << 56

	// Stalls pipeline for exactly 25 GCLK cycles.  Guarantees loading
	// pipeline is safe for use.
	Load SyncCommand = 0xf1 << 56

	// Stalls pipeline for exactly 50 GCLK cycles.  Guarantees any
	// preceeding primitives have finished rendering and it's safe to change
	// rendering modes.
	Pipe SyncCommand = 0xe7 << 56

	// Stalls pipeline for exactly 33 GCLK cycles.  Guarantees that any
	// preceding primitives have finished using tile information and
	// it's safe to modify tile descriptors.
	Tile SyncCommand = 0xe8 << 56
)

func (dl *DisplayList) sync(s SyncCommand) {
	last := SyncCommand(dl.commands[len(dl.commands)-1])
	if s == last {
		return
	}

	dl.push(Command(s))
}
