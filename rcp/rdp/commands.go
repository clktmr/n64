// This file gives direct access to some of the low-level RDP commands, which
// can be used for simple 2D graphics.  For 3D graphics the GBI interface of the
// RSP should be used.  Further documentation can be found in the official docs.
package rdp

import (
	"embedded/rtos"
	"image"
	"image/color"
	"n64/rcp/cpu"
	"runtime"
	"unsafe"
)

// Each RDP command is a 64-bit dword, but needs to be stored as two words to
// get endianess right.
type Command struct{ UW, LW uint32 }

var commandQueue []Command

func init() {
	commandQueue = make([]Command, 0, 32)
}

func Run() {
	sync(Full)

	cmds := commandQueue
	commandQueue = make([]Command, 0, 32)

	elemSize := unsafe.Sizeof(cmds[0])

	start := uintptr(unsafe.Pointer(&cmds[0]))
	end := uintptr(unsafe.Pointer(&cmds[len(cmds)-1])) + elemSize
	length := int(end - start)

	cpu.Writeback(start, length)

	for {
		if regs.status.LoadBits(startPending|endPending) == 0 {
			break
		}
		runtime.Gosched()
	}

	regs.status.Store(clrFlush | clrFreeze | clrXbus) // TODO why? see libdragon

	FullSync.Clear()

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

// properties of the current framebuffer
var fbFormat ImageFormat
var fbBbp BitDepth

// Sets the framebuffer to render the final image into.
func SetColorImage(addr uintptr, width uint32, format ImageFormat, bbp BitDepth) {
	if width > 1<<9-1 {
		return // TODO store error
	}

	cmd := (0xff << 24) | uint32(format) | uint32(bbp) | (width - 1)
	commandQueue = append(commandQueue, Command{
		UW: uint32(cmd),
		LW: cpu.PhysicalAddress(addr),
	})

	fbFormat = format
	fbBbp = bbp
}

// Sets the image where LoadTile and LoadBlock will copy their data from.
func SetTextureImage(addr uintptr, width uint32, format ImageFormat, bbp BitDepth) {
	if width > 1<<9-1 {
		return // TODO store error
	}

	cmd := (0xfd << 24) | uint32(format) | uint32(bbp) | (width - 1)
	commandQueue = append(commandQueue, Command{
		UW: uint32(cmd),
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
func SetTile(ts TileDescriptor) {
	cmdUW := 0xf5<<24 | uint32(ts.Format) | uint32(ts.Size)
	cmdUW |= uint32(ts.Line)<<9 | uint32(ts.TMEMAddr)

	cmdLW := uint32(ts.Idx)<<24 | uint32(ts.Palette)<<20
	cmdLW |= uint32(ts.MaskT)<<14 | uint32(ts.ShiftT)<<10
	cmdLW |= uint32(ts.MaskS)<<4 | uint32(ts.ShiftS)
	cmdLW |= uint32(ts.Flags)
	commandQueue = append(commandQueue, Command{
		UW: cmdUW,
		LW: cmdLW,
	})
}

// Copies a tile into TMEM.  The tile is copied from the texture image, which
// must be set prior via SetTextureImage().
func LoadTile(idx uint8, r image.Rectangle) {
	cmdUW := 0xf4<<24 | uint32(r.Min.X)<<14 | uint32(r.Min.Y)<<2
	cmdLW := uint32(idx)<<24 | uint32(r.Max.X)<<14 | uint32(r.Max.Y)<<2
	commandQueue = append(commandQueue, Command{
		UW: cmdUW,
		LW: cmdLW})
}

// Mode flags for the SetOtherModes() command.
// TODO Blend modewords (bits 16-31)
type ModeFlags uint64

const (
	AlphaCompare ModeFlags = 1 << iota
	DitherAlpha
	ZSource
	AntiAlias
	ZCompare
	ZUpdate
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

const (
	CycleTypeOne ModeFlags = iota << 52
	CycleTypeTwo
	CycleTypeCopy
	CycleTypeFill
)

const (
	RGBDitherMagicSquare ModeFlags = iota << 38
	RGBDitherBayer
	RGBDitherNoise
	RGBDitherNone
)

const (
	AlphaDitherPattern ModeFlags = iota << 36
	AlphaDitherInvPattern
	AlphaDitherNoise
	AlphaDitherNone
)

const (
	ZmodeOpaque ModeFlags = iota << 10
	ZmodeInterpenetrating
	ZmodeTransparent
	ZmodeDecal
)

const (
	CvgDestClamp ModeFlags = iota << 8
	CvgDestWrap
	CvgDestZap
	CvgDestSave
)

var lastOtherModes ModeFlags

func SetOtherModes(m ModeFlags) {
	if m == lastOtherModes {
		return // avoid costly pipeline sync
	}
	lastOtherModes = m

	sync(Pipe)

	// TODO merge with previous command if also SetOtherModes

	cmd := 0xef00_000f_0000_0000 | m
	commandQueue = append(commandQueue, Command{
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
func SetScissor(r image.Rectangle, i InterlaceFrame) {
	cmd := uint64(0xed << 56)
	cmd |= uint64(r.Min.X<<46) | uint64(r.Min.Y<<34) | uint64(r.Max.X<<14) | uint64(r.Max.Y<<2)
	cmd |= uint64(i) << 24
	commandQueue = append(commandQueue, Command{
		UW: uint32(cmd >> 32),
		LW: uint32(cmd),
	})
}

var lastFillColor color.Color

// Sets the color for the next FillRectangle() call.
func SetFillColor(c color.Color) {
	if c == lastFillColor {
		return // avoid costly pipeline sync
	}
	lastFillColor = c

	sync(Pipe)

	r, g, b, a := c.RGBA()
	var ci uint32
	if fbBbp == BBP32 {
		ci = ((r >> 8) << 24) | ((g >> 8) << 16) | ((b >> 8) << 8) | (a >> 8)
	} else if fbBbp == BBP16 {
		ci = ((r >> 11) << 11) | ((g >> 11) << 6) | ((b >> 11) << 1) | (a >> 15)
		ci |= ci << 16
	}
	commandQueue = append(commandQueue, Command{
		UW: 0xf700_0000,
		LW: ci,
	})
}

// Draws a rectangle filled with the color set by SetFillColor().
func FillRectangle(r image.Rectangle) {
	cmd := uint64(0xf6 << 56)
	cmd |= uint64(r.Max.X<<46) | uint64(r.Max.Y<<34) | uint64(r.Min.X<<14) | uint64(r.Min.Y<<2)
	commandQueue = append(commandQueue, Command{
		UW: uint32(cmd >> 32),
		LW: uint32(cmd),
	})
}

// Draws a textured rectangle.
func TextureRectangle(r image.Rectangle, tileIdx uint8) {
	cmdUW := 0xe4<<24 | uint32(r.Max.X)<<14 | uint32(r.Max.Y)<<2
	cmdLW := uint32(tileIdx)<<24 | uint32(r.Min.X)<<14 | uint32(r.Min.Y)<<2
	commandQueue = append(commandQueue, []Command{
		Command{UW: cmdUW, LW: cmdLW},
		Command{UW: uint32(0), LW: uint32(1<<28 | 1<<10)},
	}...)
}

type SyncCommand uint32

const (
	// Waits until all previous commands have finished reading and writing
	// to RDRAM.  Additionally raises the RDP interrupt.  Use to sync memory
	// access between RDP and other components (e.g. switching framebuffers) or when changing RDPs RDRAM
	// buffers (e.g. Render to texture).
	Full SyncCommand = 0xe900_0000
	Load SyncCommand = 0xf100_0000
	Pipe SyncCommand = 0xe700_0000

	// Writing to a tile waits until an immediately previous command finished
	// reading from the tile.
	Tile SyncCommand = 0xe800_0000
)

func sync(s SyncCommand) {
	commandQueue = append(commandQueue, Command{
		UW: uint32(s),
		LW: 0x0,
	})
}

var FullSync rtos.Note
var IrqCnt uint

//go:nosplit
//go:nowritebarrierrec
func Handler() {
	IrqCnt += 1
	FullSync.Wakeup()
}
