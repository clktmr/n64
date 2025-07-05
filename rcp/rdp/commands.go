package rdp

import (
	"image"
	"image/color"
	"time"
	"unsafe"

	"github.com/clktmr/n64/debug"
	"github.com/clktmr/n64/rcp/cpu"
	"github.com/clktmr/n64/rcp/texture"
)

type command uint64

const (
	// Waits until all previous commands have finished reading and writing
	// to RDRAM. Additionally raises the RDP interrupt.  Use to sync memory
	// access between RDP and other components (e.g. switching framebuffers)
	// or when changing RDPs RDRAM buffers (e.g. Render to texture).
	SyncFull command = 0xe9 << 56

	// Stalls pipeline for exactly 25 GCLK cycles. Guarantees loading
	// pipeline is safe for use.
	SyncLoad command = 0xf1 << 56

	// Stalls pipeline for exactly 50 GCLK cycles. Guarantees any
	// preceeding primitives have finished rendering and it's safe to change
	// rendering modes.
	SyncPipe command = 0xe7 << 56

	// Stalls pipeline for exactly 33 GCLK cycles. Guarantees that any
	// preceding primitives have finished using tile information and
	// it's safe to modify tile descriptors.
	SyncTile command = 0xe8 << 56
)

type DisplayList struct {
	state

	// TODO Pin buffers
	buf [2]struct {
		_        cpu.CacheLinePad
		commands [512]command
		_        cpu.CacheLinePad
	}
	bufIdx     int
	start, end uintptr

	pinner cpu.Pinner
}

type state struct {
	combineMode      CombineMode
	otherModes       ModeFlags
	fillColor        color.RGBA
	blendColor       color.NRGBA
	primitiveColor   color.NRGBA
	environmentColor color.NRGBA

	scissorSet, scissorReal image.Rectangle
	interlace               InterlaceFrame

	// framebuffer info
	addr cpu.Addr
	size image.Point
	bpp  texture.Depth
}

var RDP DisplayList

func init() {
	RDP.start = uintptr(unsafe.Pointer(&RDP.buf[RDP.bufIdx].commands))
	RDP.end = RDP.start

	regs().status.Store(clrFlush | clrFreeze | clrXbus)
	regs().start.Store(cpu.PAddr(RDP.start))
	regs().end.Store(cpu.PAddr(RDP.end))
}

// Flush blocks until all enqueued commands are fully processed. After a Flush,
// commands must not rely on any state from before the flush.
func (dl *DisplayList) Flush() {
	dl.Push(SyncFull)
	if !fullSync.Wait(1 * time.Second) {
		panic("rdp timeout")
	}
	dl.addr = 0x0
	dl.pinner.Unpin()
}

//go:nosplit
func (dl *DisplayList) Push(cmds ...command) { // TODO unexport
	for _, cmd := range cmds {
		// unsafe.Pointer is used for performance
		ptr := cpu.Uncached((*command)(unsafe.Pointer(dl.end)))
		*ptr = cmd

		dl.end += 8

		if int(dl.end-dl.start) == len(dl.buf[0].commands)<<3 {
			regs().end.Store(cpu.PAddr(dl.end))
			dl.bufIdx = 1 - dl.bufIdx
			dl.start = uintptr(unsafe.Pointer(&dl.buf[dl.bufIdx].commands))
			dl.end = dl.start
			for retries := 0; regs().status.LoadBits(startPending) != 0; retries++ {
				if retries > 1024*1024 { // wait max ~1 sec
					panic("rdp stall")
				}
			}
			regs().start.Store(cpu.PAddr(dl.start))
			regs().end.Store(cpu.PAddr(dl.start))
		}
	}
	regs().end.Store(cpu.PAddr(dl.end))
}

// Sets the framebuffer to render the final image into.
func (dl *DisplayList) SetColorImage(img *texture.Texture) {
	debug.Assert(img.Addr()%64 == 0, "rdp framebuffer alignment")
	debug.Assert(img.Stride() < 1<<10, "rdp framebuffer width too big")
	debug.Assert(img.Format() == texture.RGBA16 ||
		img.Format() == texture.RGBA32 ||
		img.Format() == texture.I8, "rdp unsupported format")

	if dl.addr == img.Addr() {
		return
	}

	cpu.PinSlice(&dl.pinner, img.Pix())

	dl.Push(((0xff << 56) | command(img.Format()) | command(img.Stride()-1)<<32) |
		command(img.Addr()))

	dl.addr = img.Addr()
	dl.size = img.Bounds().Size()
	dl.bpp = img.Format().Depth()
}

// Sets the zbuffer. Width is taken from SetColorImage, bpp is always 18.
func (dl *DisplayList) SetDepthImage(img *texture.Texture) {
	debug.Assert(img.Addr()%64 == 0, "rdp zbuffer alignment")

	cpu.PinSlice(&dl.pinner, img.Pix())

	dl.Push(command((0xfe << 56) | command(img.Addr())))
}

// Sets the image where LoadTile and LoadBlock will copy their data from.
func (dl *DisplayList) SetTextureImage(img *texture.Texture) {
	debug.Assert(img.Addr()%8 == 0, "rdp texture must be 8 byte aligned")
	debug.Assert(img.Stride() <= 1<<9, "rdp texture width too big")

	format := img.Format()
	stride := img.Stride()
	if format.Depth() == texture.BPP4 { // loading BPP4 crashes the RDP
		format = texture.Format(format.Components()) | texture.Format(texture.BPP8)
		stride >>= 1
	}

	cpu.PinSlice(&dl.pinner, img.Pix())

	// according to wiki, format[23:21] has no effect
	dl.Push((0xfd << 56) | command(format) | command(stride-1)<<32 |
		command(img.Addr()))
}

type TileDescFlags uint64

const (
	MirrorS TileDescFlags = 1 << 8
	ClampS  TileDescFlags = 1 << 9
	MirrorT TileDescFlags = 1 << 18
	ClampT  TileDescFlags = 1 << 19
)

func supportedFormat(fmt texture.Format) bool {
	bpp := fmt.Depth()
	switch fmt.Components() {
	case texture.RGBA:
		return bpp == texture.BPP16 || bpp == texture.BPP32
	case texture.YUV:
		return bpp == texture.BPP16
	case texture.IA:
		return bpp == texture.BPP4 || bpp == texture.BPP8 || bpp == texture.BPP16
	case texture.I:
		fallthrough
	case texture.CI:
		return bpp == texture.BPP4 || bpp == texture.BPP8
	}
	return false
}

const (
	tile4bpp = 1
)

type TileDescriptor struct {
	Format         texture.Format
	Line           uint16 // 9 bit; line length in qwords
	Addr           uint16 // 9 bit; TMEM address in qwords
	idx            uint8  // 3 bit; Tile index
	Palette        uint8  // 4 bit; Palette index
	MaskT, MaskS   uint8  // 4 bit
	ShiftT, ShiftS uint8  // 4 bit
	Flags          TileDescFlags
}

// Sets a tile's properties. There are a total of eight tiles, identified by
// the Idx field, which can later be referenced in other commands, e.g.
// LoadTile().
func (dl *DisplayList) SetTile(ts TileDescriptor) (loadIdx, drawIdx uint8) {
	debug.Assert(ts.Line < 1<<9, "tile line length out of bounds")
	debug.Assert(ts.Addr < 1<<9, "tile addr out of bounds")
	debug.Assert(ts.idx < 1<<3, "tile index out of bounds")
	debug.Assert(ts.Palette < 1<<4, "tile palette index out of bounds")
	debug.Assert(ts.MaskT < 1<<4, "tile mask out of bounds")
	debug.Assert(ts.MaskS < 1<<4, "tile mask out of bounds")
	debug.Assert(ts.ShiftT < 1<<4, "tile shift out of bounds")
	debug.Assert(ts.ShiftS < 1<<4, "tile shift out of bounds")
	debug.Assert(supportedFormat(ts.Format), "tile unsupported format")

	// some formats must indicate 16 byte instead of 8 byte texels
	if ts.Format.Depth() == texture.BPP32 {
		ts.Line = ts.Line >> 1
	}

	if ts.Format.Depth() == texture.BPP4 {
		// Loading BPP4 crashes the RDP. As a workaround create two tiles with
		// different BPP, one for loading and one for drawing.
		loadIdx = tile4bpp
		tsload := ts
		tsload.idx = loadIdx
		tsload.Format = tsload.Format.SetDepth(texture.BPP8)
		dl.SetTile(tsload)
	}

	cmd := command(0xf5<<56) | command(ts.Format)
	cmd |= command(ts.Line)<<41 | command(ts.Addr)<<32
	cmd |= command(ts.idx)<<24 | command(ts.Palette)<<20
	cmd |= command(ts.MaskT)<<14 | command(ts.ShiftT)<<10
	cmd |= command(ts.MaskS)<<4 | command(ts.ShiftS)
	cmd |= command(ts.Flags)

	dl.Push(SyncTile, cmd)

	return
}

// Copies a tile into TMEM. The tile is copied from the texture image, which
// must be set prior via SetTextureImage().
func (dl *DisplayList) LoadTile(idx uint8, r image.Rectangle) {
	if idx == tile4bpp {
		r.Min.X &^= 0x1
		r.Max.X = (r.Max.X + 1) &^ 0x1
		dl.SetTileSize(0, r)
		r.Min.X >>= 1
		r.Max.X >>= 1
	}

	cmd := 0xf4<<56 | command(r.Min.X)<<46 | command(r.Min.Y)<<34
	cmd |= command(idx)<<24 | command(r.Max.X-1)<<14 | command(r.Max.Y-1)<<2

	dl.Push(SyncTile, cmd)
}

// Copies a color palette into TMEM. The palette is copied from the texture image, which
// must be set prior via SetTextureImage().
func (dl *DisplayList) LoadTLUT(idx uint8, r image.Rectangle) {
	cmd := 0xf0<<56 | command(r.Min.X)<<46 | command(r.Min.Y)<<34
	cmd |= command(idx)<<24 | command(r.Max.X-1)<<14 | command(r.Max.Y-1)<<2

	dl.Push(cmd)
}

// Tile size is automatically set on LoadTile(), but can be overidden with
// SetTileSize().
func (dl *DisplayList) SetTileSize(idx uint8, r image.Rectangle) {
	cmd := 0xf2<<56 | command(r.Min.X)<<46 | command(r.Min.Y)<<34
	cmd |= command(idx)<<24 | command(r.Max.X-1)<<14 | command(r.Max.Y-1)<<2

	dl.Push(cmd)
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
	debug.Assert(!(ct == CycleTypeCopy && dl.bpp == texture.BPP32), "COPY mode unavailable for 32-bit framebuffer")
	debug.Assert(!(ct == CycleTypeFill && dl.bpp == texture.BPP4), "FILL mode unavailable for 4-bit framebuffer")

	m := flags | blend.modeFlags()
	m |= ModeFlags(ct) | ModeFlags(cDith) | ModeFlags(aDith) | ModeFlags(zMode) | ModeFlags(cvgDest)

	if m == dl.otherModes {
		return
	}
	dl.otherModes = m

	dl.SetScissor(dl.scissorSet, dl.interlace)

	cmd := 0xef00_000f_0000_0000 | m
	dl.Push(SyncPipe, command(cmd))
}

type InterlaceFrame uint64

const (
	InterlaceNone InterlaceFrame = iota << 24 // draw all lines
	_
	InterlaceOdd  // skip odd lines
	InterlaceEven // skip even lines
)

// Everything outside `r` is skipped when rendering. Additionally odd or even
// lines can be skipped to render interlaced frames.
func (dl *DisplayList) SetScissor(r image.Rectangle, il InterlaceFrame) {
	dl.scissorSet = r

	if dl.otherModes&ModeFlags(CycleTypeCopy|CycleTypeFill) != 0 {
		r.Max = r.Max.Sub(image.Point{1, 0})
	}

	if r == dl.scissorReal && il == dl.interlace {
		return
	}
	dl.scissorReal = r
	dl.interlace = il

	cmd := 0xed<<56 | command(il)
	cmd |= command(r.Min.X<<46) | command(r.Min.Y<<34) | command(r.Max.X<<14) | command(r.Max.Y<<2)

	dl.Push(command(cmd))
}

// Sets the color for subsequent FillRectangle() calls.
func (dl *DisplayList) SetFillColor(c color.Color) {
	cRGBA := asRGBA(c)
	if cRGBA == dl.fillColor {
		return
	}
	dl.fillColor = cRGBA

	r, g, b, a := uint32(dl.fillColor.R), uint32(dl.fillColor.G), uint32(dl.fillColor.B), uint32(dl.fillColor.A)
	var ci uint32
	if dl.bpp == texture.BPP32 {
		ci = (r << 24) | (g << 16) | (b << 8) | a
	} else if dl.bpp == texture.BPP16 {
		ci = ((r >> 3) << 11) | ((g >> 3) << 6) | ((b >> 3) << 1) | (a >> 15)
		ci |= ci << 16
	} else if dl.bpp == texture.BPP8 {
		ci = (a << 24) | (a << 16) | (a << 8) | a
	} else {
		debug.Assert(false, "fill color unavailable for 4-bit framebuffer")
	}
	dl.Push(SyncPipe, command(0xf7<<56)|command(ci))
}

func (dl *DisplayList) SetBlendColor(c color.Color) {
	cNRGBA := asNRGBA(c)
	if cNRGBA == dl.blendColor {
		return
	}
	dl.blendColor = cNRGBA

	dl.Push(SyncPipe, 0xf9<<56|
		command(dl.blendColor.R)<<24|command(dl.blendColor.G)<<16|
		command(dl.blendColor.B)<<8|command(dl.blendColor.A))
}

func (dl *DisplayList) SetPrimitiveColor(c color.Color) {
	cNRGBA := asNRGBA(c)
	if cNRGBA == dl.primitiveColor {
		return
	}
	dl.primitiveColor = cNRGBA

	dl.Push(0xfa<<56 |
		command(dl.primitiveColor.R)<<24 | command(dl.primitiveColor.G)<<16 |
		command(dl.primitiveColor.B)<<8 | command(dl.primitiveColor.A))
}

func (dl *DisplayList) SetEnvironmentColor(c color.Color) {
	cNRGBA := asNRGBA(c)
	if cNRGBA == dl.environmentColor {
		return
	}
	dl.environmentColor = cNRGBA

	dl.Push(SyncPipe, 0xfb<<56|
		command(dl.environmentColor.R)<<24|command(dl.environmentColor.G)<<16|
		command(dl.environmentColor.B)<<8|command(dl.environmentColor.A))
}

func (dl *DisplayList) SetCombineMode(m CombineMode) {
	if dl.combineMode == m {
		return
	}
	dl.combineMode = m

	cmd := command(0xfc<<56 |
		m.One.RGB.A<<52 | m.One.RGB.C<<47 |
		m.One.Alpha.A<<44 | m.One.Alpha.C<<41 |
		m.Two.RGB.A<<37 | m.Two.RGB.C<<32)
	cmd |= command(0x0 |
		m.One.RGB.B<<28 | m.Two.RGB.B<<24 |
		m.Two.Alpha.A<<21 | m.Two.Alpha.C<<18 |
		m.One.RGB.D<<15 | m.One.Alpha.B<<12 | m.One.Alpha.D<<9 |
		m.Two.RGB.D<<6 | m.Two.Alpha.B<<3 | m.Two.Alpha.D)

	dl.Push(SyncPipe, cmd)
}

// Draws a rectangle filled with the color set by SetFillColor().
func (dl *DisplayList) FillRectangle(r image.Rectangle) {
	r = r.Intersect(image.Rectangle{Max: dl.size})

	if dl.otherModes&ModeFlags(CycleTypeCopy|CycleTypeFill) != 0 {
		r.Max = r.Max.Sub(image.Point{1, 1})
	}

	cmd := 0xf6<<56 | command(r.Max.X<<46) | command(r.Max.Y<<34)
	cmd |= command(r.Min.X<<14) | command(r.Min.Y<<2)
	dl.Push(cmd)
}

// Draws a textured rectangle.
func (dl *DisplayList) TextureRectangle(r image.Rectangle, p image.Point, scale image.Point, tileIdx uint8) {
	full := r
	r = r.Intersect(image.Rectangle{Max: dl.size})
	p = p.Add(r.Min.Sub(full.Min))

	if dl.otherModes&ModeFlags(CycleTypeCopy|CycleTypeFill) != 0 {
		r.Max = r.Max.Sub(image.Point{1, 1})
	}

	cmd := 0xe4<<56 | command(r.Max.X)<<46 | command(r.Max.Y)<<34
	cmd |= command(tileIdx)<<24 | command(r.Min.X)<<14 | command(r.Min.Y)<<2
	cmd2 := command(p.X<<53) | command(p.Y<<37)
	cmd2 |= command(((0x8000/scale.X)>>5)<<16 | (0x8000/scale.Y)>>5)
	dl.Push(cmd, cmd2)
}

// MaxTileSize returns the largest tile that fits into TMEM for the given
// bitdepth.
func MaxTileSize(format texture.Format) image.Rectangle {
	var x, y int
	switch format.Depth() {
	case texture.BPP4:
		x, y = 64, 128
	case texture.BPP8:
		x, y = 64, 64
	case texture.BPP16:
		x, y = 32, 64
	case texture.BPP32:
		x, y = 32, 32
	default:
		panic("invalid bpp")
	}

	if format.Components() == texture.CI {
		y >>= 1
	}

	return image.Rect(0, 0, x, y)
}

// Little unsafe helper to avoid heap allocation from calls to RGBA().
//
//go:noescape
//go:linkname asRGBA github.com/clktmr/n64/rcp/rdp._asRGBA
func asRGBA(c color.Color) (ret color.RGBA)
func _asRGBA(c color.Color) (ret color.RGBA) {
	if c, ok := c.(color.RGBA); ok {
		return c
	}
	r, g, b, a := c.RGBA()
	ret.R = uint8(r >> 8)
	ret.G = uint8(g >> 8)
	ret.B = uint8(b >> 8)
	ret.A = uint8(a >> 8)
	return
}

// Little unsafe helper to avoid heap allocation from calls to RGBA().
//
//go:noescape
//go:linkname asNRGBA github.com/clktmr/n64/rcp/rdp._asNRGBA
func asNRGBA(c color.Color) (ret color.NRGBA)
func _asNRGBA(c color.Color) (ret color.NRGBA) {
	if c, ok := c.(color.NRGBA); ok {
		return c
	}
	r, g, b, a := c.RGBA()
	if a == 0xffff {
		return color.NRGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), 0xff}
	}
	if a == 0 {
		return color.NRGBA{0, 0, 0, 0}
	}
	r = (r * 0xffff) / a
	g = (g * 0xffff) / a
	b = (b * 0xffff) / a
	return color.NRGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
}
