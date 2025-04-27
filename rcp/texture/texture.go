package texture

import (
	"image"
	"image/draw"

	"github.com/clktmr/n64/rcp/cpu"
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
	BPP4 BitDepth = iota << 51
	BPP8
	BPP16
	BPP32
)

// For a number of pixels returns their size in bytes.
func (bpp BitDepth) Bytes(pixels int) int {
	shift := int(bpp)>>51 - 1
	if shift < 0 {
		return pixels >> -shift
	}
	return pixels << shift
}

type Texture interface {
	image.Image

	// Addr returns the base address of the images pixel data.
	Addr() cpu.Addr

	// Stride returns the stride (in pixels) between vertically adjacent
	// pixels.
	Stride() int

	// Format returns the pixel's color format.
	Format() ImageFormat

	// BPP returns the pixel's bit depth.
	BPP() BitDepth

	// Premult returns whether the image's color channels are premultiplied
	// with it's alpha channels.
	Premult() bool
}

// ImageTexture is a texture that provides a draw.Image implementation. This is
// useful to get the underlying image type when passing to the stdlib, e.g.
// draw.DrawMask(). The image/draw will use optimized implementations via type
// assertions, so it's important to pass an image type image/draw knows.
type ImageTexture interface {
	Image() draw.Image
}
