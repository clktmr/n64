package texture

import (
	"image"
	"image/draw"
)

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

type Texture interface {
	Bounds() image.Rectangle
	Addr() uintptr
	Stride() int
	Format() ImageFormat
	BPP() BitDepth
	Premult() bool
}

// A texture that provides a draw.Image implementation.  This is useful to get
// the underlying image type when passing to the stdlib, e.g. draw.DrawMask().
// The image/draw will use optimized implementations via type assertions, so
// it's important to pass an image type image/draw knows.
type ImageTexture interface {
	Image() draw.Image
}

type CachedTexture interface {
	Writeback()
	Invalidate()
}

// For a number of pixels returns their size in bytes.
func PixelsToBytes(pixels int, bpp BitDepth) int {
	shift := int(bpp)>>51 - 1
	if shift < 0 {
		return pixels >> -shift
	}
	return pixels << shift
}
