// Package texture provides image types used by the rcp, e.g. textures and
// framebuffers.
package texture

import (
	"image"
	"image/draw"
	"unsafe"

	"github.com/clktmr/n64/rcp/cpu"
)

// TODO ensure alignment in New*FromImage() and *.SubImage()

type Format uint64

const (
	RGBA32 = Format(RGBA) | Format(BPP32)
	RGBA16 = Format(RGBA) | Format(BPP16)
	YUV16  = Format(YUV) | Format(BPP16)
	IA16   = Format(IA) | Format(BPP16)
	IA8    = Format(IA) | Format(BPP8)
	IA4    = Format(IA) | Format(BPP4)
	I8     = Format(I) | Format(BPP8)
	I4     = Format(I) | Format(BPP4)
	CI8    = Format(CI) | Format(BPP8)
	CI4    = Format(CI) | Format(BPP4)
)

func (c Format) Components() Components {
	return Components(c & (0x7 << 53))
}

func (c Format) Depth() Depth {
	return Depth(c & (0x3 << 51))
}

func (c Format) SetDepth(bpp Depth) Format {
	return Format(c.Components()) | Format(bpp)
}

// TMEMWords returns the size in TMEM words (8 byte) for a number of pixels.
func (c Format) TMEMWords(pixels int) int {
	return (c.Bits(pixels) + 63) >> 6
}

// Bytes returns the size in bytes for a number of pixels.
func (c Format) Bytes(pixels int) int {
	return (c.Bits(pixels) + 7) >> 3
}

// Bits returns the size in bits for a number of pixels.
func (c Format) Bits(pixels int) int {
	shift := int(c.Depth())>>51 + 2
	return pixels << shift
}

type Components uint64

const (
	RGBA Components = iota << 53
	YUV
	CI // Color Palette
	IA // Intensity with alpha
	I  // Intensity
)

type Depth uint64

const (
	BPP4 Depth = iota << 51
	BPP8
	BPP16
	BPP32
)

const (
	alignFramebuffer = 64
	alignTexture     = 8
)

type Texture struct {
	draw.Image

	pix    []byte
	stride int
	rect   *image.Rectangle

	format  Format
	premult bool
	palette *Texture
}

// Addr returns the base address of the images pixel data.
func (p *Texture) Addr() cpu.Addr { return cpu.PhysicalAddressSlice(p.pix) }

// Addr returns a pointer to the images pixel data.
func (p *Texture) Pointer() unsafe.Pointer { return unsafe.Pointer(unsafe.SliceData(p.pix)) }

// Stride returns the stride (in pixels) between vertically adjacent
// pixels.
func (p *Texture) Stride() int { return p.stride }

// Format returns the pixel's color format.
func (p *Texture) Format() Format { return p.format }

// Palette returns the color palette texture for formats CI4 and CI8.
func (p *Texture) Palette() *Texture { return p.palette }

// SetOrigin moves the coordinate system of the texture to origin. Useful if
// subimages are used as viewports and should have their origin in (0, 0).
func (p *Texture) SetOrigin(origin image.Point) { *p.rect = p.rect.Sub(p.rect.Min.Sub(origin)) }

// Premult returns whether the image's color channels are premultiplied
// with it's alpha channels.
func (p *Texture) Premult() bool { return p.premult }
func (p *Texture) Writeback()    { cpu.WritebackSlice(p.pix) }
func (p *Texture) Invalidate()   { cpu.InvalidateSlice(p.pix) }
func (p *Texture) SubImage(r image.Rectangle) *Texture {
	sub, _ := p.Image.(interface {
		SubImage(r image.Rectangle) image.Image
	})
	return newTextureFromImage(sub.SubImage(r))
}

func newTextureFromImage(img image.Image) (tex *Texture) {
	switch img := img.(type) {
	case *image.RGBA:
		tex = &Texture{img, img.Pix, img.Stride >> 2, &img.Rect, RGBA32, true, nil}
	case *image.NRGBA:
		tex = &Texture{img, img.Pix, img.Stride >> 2, &img.Rect, RGBA32, false, nil}
	case *imageRGBA16:
		tex = &Texture{img, img.Pix, img.Stride >> 1, &img.Rect, RGBA16, true, nil}
	case *image.Alpha:
		tex = &Texture{img, img.Pix, img.Stride, &img.Rect, I8, false, nil}
	case *image.Gray:
		tex = &Texture{img, img.Pix, img.Stride, &img.Rect, I8, true, nil}
	case *imageI4:
		tex = &Texture{img, img.Pix, img.Stride << 1, &img.Rect, I4, true, nil}
	case *imageCI8:
		tex = &Texture{img, img.Pix, img.Stride, &img.Rect, CI8, true, NewTextureFromImage(img.Palette)}
	default:
		panic("unsupported image format")
	}
	return
}

func NewTextureFromImage(img image.Image) (tex *Texture) {
	tex = newTextureFromImage(img)
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
	return NewTextureFromImage(&image.Gray{
		Pix:    cpu.MakePaddedSliceAligned[byte](r.Dx()*r.Dy(), alignFramebuffer),
		Stride: r.Dx(),
		Rect:   r,
	})
}

// Stores pixels alpha with 8bit
func NewAlpha(r image.Rectangle) *Texture {
	return NewTextureFromImage(&image.Alpha{
		Pix:    cpu.MakePaddedSliceAligned[byte](r.Dx()*r.Dy(), alignFramebuffer),
		Stride: r.Dx(),
		Rect:   r,
	})
}

// Stores pixels intensity with 4bit
func NewI4(r image.Rectangle) *Texture {
	dx := (r.Dx() + 0x1) &^ 0x1
	return NewTextureFromImage(&imageI4{
		Pix:    cpu.MakePaddedSliceAligned[byte](dx*r.Dy()/2, alignFramebuffer),
		Stride: dx / 2,
		Rect:   r,
	})
}

// Stores pixels color index in a RGBA16 palette
func NewCI8(r image.Rectangle, palette *ColorPalette) *Texture {
	return NewTextureFromImage(&imageCI8{
		Pix:     cpu.MakePaddedSliceAligned[byte](r.Dx()*r.Dy(), alignFramebuffer),
		Stride:  r.Dx(),
		Rect:    r,
		Palette: palette,
	})
}
