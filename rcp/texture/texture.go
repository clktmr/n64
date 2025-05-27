// Package texture provides image types used by the rcp, e.g. textures and
// framebuffers.
package texture

import (
	"image"
	"image/draw"

	"github.com/clktmr/n64/rcp/cpu"
)

// TODO ensure alignment in New*FromImage() and *.SubImage()

type ImageFormat uint64

const (
	RGBA ImageFormat = iota << 53
	YUV
	CI // Color Palette
	IA // Intensity with alpha
	I  // Intensity
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

const (
	alignFramebuffer = 64
	alignTexture     = 8
)

type Texture struct {
	draw.Image

	pix     []byte
	stride  int
	format  ImageFormat
	bpp     BitDepth
	premult bool
}

// Addr returns the base address of the images pixel data.
func (p *Texture) Addr() cpu.Addr { return cpu.PhysicalAddressSlice(p.pix) }

// Stride returns the stride (in pixels) between vertically adjacent
// pixels.
func (p *Texture) Stride() int { return p.stride }

// Format returns the pixel's color format.
func (p *Texture) Format() ImageFormat { return p.format }

// BPP returns the pixel's bit depth.
func (p *Texture) BPP() BitDepth { return p.bpp }

// Premult returns whether the image's color channels are premultiplied
// with it's alpha channels.
func (p *Texture) Premult() bool { return p.premult }
func (p *Texture) Writeback()    { cpu.WritebackSlice(p.pix) }
func (p *Texture) Invalidate()   { cpu.InvalidateSlice(p.pix) }
func (p *Texture) SubImage(r image.Rectangle) *Texture {
	sub, _ := p.Image.(interface {
		SubImage(r image.Rectangle) image.Image
	})
	return NewTextureFromImage(sub.SubImage(r))
}

func NewTextureFromImage(img image.Image) (tex *Texture) {
	switch img := img.(type) {
	case *image.RGBA:
		tex = &Texture{img, img.Pix, img.Stride >> 2, RGBA, BPP32, true}
	case *image.NRGBA:
		tex = &Texture{img, img.Pix, img.Stride >> 2, RGBA, BPP32, false}
	case *imageRGBA16:
		tex = &Texture{img, img.Pix, img.Stride >> 1, RGBA, BPP16, true}
	case *image.Alpha: // TODO should be *image.Gray?
		tex = &Texture{img, img.Pix, img.Stride, I, BPP8, false}
	}
	tex.Writeback()
	return
}

// Stores pixels in RGBA with 32bit (8:8:8:8)
func NewRGBA32(r image.Rectangle) *Texture {
	return NewTextureFromImage(&image.RGBA{
		Pix:    cpu.MakePaddedSliceAligned[byte](r.Dx()*r.Dy()*4, alignFramebuffer),
		Stride: 4 * r.Dx(),
		Rect:   r,
	})
}

// Stores pixels in RGBA with 32bit (8:8:8:8)
//
// Same as RGBA32, but not premultiplied-alpha.
func NewNRGBA32(r image.Rectangle) *Texture {
	return NewTextureFromImage(&image.NRGBA{
		Pix:    cpu.MakePaddedSliceAligned[byte](r.Dx()*r.Dy()*4, alignFramebuffer),
		Stride: 4 * r.Dx(),
		Rect:   r,
	})
}

// Stores pixels in RGBA with 16bit (5:5:5:1)
func NewRGBA16(r image.Rectangle) *Texture {
	return NewTextureFromImage(&imageRGBA16{
		Pix:    cpu.MakePaddedSliceAligned[byte](r.Dx()*r.Dy()*2, alignFramebuffer),
		Stride: 2 * r.Dx(),
		Rect:   r,
	})
}

// Stores pixels intensity with 8bit
func NewI8(r image.Rectangle) *Texture {
	return NewTextureFromImage(&image.Alpha{
		Pix:    cpu.MakePaddedSliceAligned[byte](r.Dx()*r.Dy(), alignFramebuffer),
		Stride: r.Dx(),
		Rect:   r,
	})
}
