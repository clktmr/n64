// Package texture provides a common datastructure for images used by the rcp,
// e.g. textures and framebuffers.
package texture

// TODO ensure alignment in New*FromImage() and *.SubImage()

import (
	"image"
	"image/draw"

	"github.com/clktmr/n64/rcp/cpu"
)

const (
	AlignFramebuffer = 64
	AlignTexture     = 8
)

// Stores pixels in RGBA with 32bit (8:8:8:8)
type RGBA32 struct{ image.RGBA }

func NewRGBA32(r image.Rectangle) *RGBA32 {
	return &RGBA32{image.RGBA{
		Pix:    cpu.MakePaddedSliceAligned[byte](r.Dx()*r.Dy()*4, AlignFramebuffer),
		Stride: 4 * r.Dx(),
		Rect:   r,
	}}
}

func NewRGBA32FromImage(img *image.RGBA) *RGBA32 {
	tex := &RGBA32{*img}
	tex.Writeback()
	return tex
}

func (p *RGBA32) Image() draw.Image   { return &p.RGBA }
func (p *RGBA32) Addr() cpu.Addr      { return cpu.PhysicalAddressSlice(p.Pix) }
func (p *RGBA32) Stride() int         { return p.RGBA.Stride >> 2 }
func (p *RGBA32) Format() ImageFormat { return RGBA }
func (p *RGBA32) BPP() BitDepth       { return BPP32 }
func (p *RGBA32) Premult() bool       { return true }
func (p *RGBA32) Writeback()          { cpu.WritebackSlice(p.Pix) }
func (p *RGBA32) Invalidate()         { cpu.InvalidateSlice(p.Pix) }

func (p *RGBA32) SubImage(r image.Rectangle) *RGBA32 {
	subImg, _ := p.RGBA.SubImage(r).(*image.RGBA)
	return &RGBA32{*subImg}
}

// Stores pixels in RGBA with 32bit (8:8:8:8)
//
// Same as RGBA32, but not premultiplied-alpha.
type NRGBA32 struct{ image.NRGBA }

func NewNRGBA32(r image.Rectangle) *NRGBA32 {
	return &NRGBA32{image.NRGBA{
		Pix:    cpu.MakePaddedSliceAligned[byte](r.Dx()*r.Dy()*4, AlignFramebuffer),
		Stride: 4 * r.Dx(),
		Rect:   r,
	}}
}

func NewNRGBA32FromImage(img *image.NRGBA) *NRGBA32 {
	tex := &NRGBA32{*img}
	tex.Writeback()
	return tex
}

func (p *NRGBA32) Image() draw.Image   { return &p.NRGBA }
func (p *NRGBA32) Addr() cpu.Addr      { return cpu.PhysicalAddressSlice(p.Pix) }
func (p *NRGBA32) Stride() int         { return p.NRGBA.Stride >> 2 }
func (p *NRGBA32) Format() ImageFormat { return RGBA }
func (p *NRGBA32) BPP() BitDepth       { return BPP32 }
func (p *NRGBA32) Premult() bool       { return false }
func (p *NRGBA32) Writeback()          { cpu.WritebackSlice(p.Pix) }
func (p *NRGBA32) Invalidate()         { cpu.InvalidateSlice(p.Pix) }

func (p *NRGBA32) SubImage(r image.Rectangle) *NRGBA32 {
	subImg, _ := p.NRGBA.SubImage(r).(*image.NRGBA)
	return &NRGBA32{*subImg}
}

// Stores pixels in RGBA with 16bit (5:5:5:1)
type RGBA16 struct{ imageRGBA16 }

func NewRGBA16(r image.Rectangle) *RGBA16 {
	return &RGBA16{imageRGBA16{
		Pix:    cpu.MakePaddedSliceAligned[byte](r.Dx()*r.Dy()*2, AlignFramebuffer),
		Stride: 2 * r.Dx(),
		Rect:   r,
	}}
}

func NewRGBA16FromImage(img *imageRGBA16) *RGBA16 {
	tex := &RGBA16{*img}
	tex.Writeback()
	return tex
}

func (p *RGBA16) Image() draw.Image   { return &p.imageRGBA16 }
func (p *RGBA16) Addr() cpu.Addr      { return cpu.PhysicalAddressSlice(p.Pix) }
func (p *RGBA16) Stride() int         { return p.imageRGBA16.Stride >> 1 }
func (p *RGBA16) Format() ImageFormat { return RGBA }
func (p *RGBA16) BPP() BitDepth       { return BPP16 }
func (p *RGBA16) Premult() bool       { return true }
func (p *RGBA16) Writeback()          { cpu.WritebackSlice(p.Pix) }
func (p *RGBA16) Invalidate()         { cpu.InvalidateSlice(p.Pix) }

// Stores pixels intensity with 8bit
type I8 struct{ image.Alpha }

func NewI8(r image.Rectangle) *I8 {
	return &I8{image.Alpha{
		Pix:    cpu.MakePaddedSliceAligned[byte](r.Dx()*r.Dy(), AlignFramebuffer),
		Stride: r.Dx(),
		Rect:   r,
	}}
}

func NewI8FromImage(img *image.Alpha) *I8 {
	tex := &I8{*img}
	tex.Writeback()
	return tex
}

func (p *I8) Image() draw.Image   { return &p.Alpha }
func (p *I8) Addr() cpu.Addr      { return cpu.PhysicalAddressSlice(p.Pix) }
func (p *I8) Stride() int         { return p.Alpha.Stride }
func (p *I8) Format() ImageFormat { return I }
func (p *I8) BPP() BitDepth       { return BPP8 }
func (p *I8) Premult() bool       { return false }
func (p *I8) Writeback()          { cpu.WritebackSlice(p.Pix) }
func (p *I8) Invalidate()         { cpu.InvalidateSlice(p.Pix) }

func (p *I8) SubImage(r image.Rectangle) *I8 {
	subImg, _ := p.Alpha.SubImage(r).(*image.Alpha)
	return &I8{*subImg}
}
