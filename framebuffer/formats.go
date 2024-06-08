package framebuffer

import (
	"image"
	"image/color"
	"n64/rcp/cpu"
)

const Alignment = 64

func NewRGBA32(r image.Rectangle) *image.RGBA {
	return &image.RGBA{
		Pix:    cpu.MakePaddedSliceAligned[byte](r.Dx()*r.Dy()*4, Alignment),
		Stride: 4 * r.Dx(),
		Rect:   r,
	}
}

// Stores pixels in RGBA with 16bit (5:5:5:1)
type RGBA16 struct {
	Pix    []uint8
	Stride int
	Rect   image.Rectangle
}

func NewRGBA16(r image.Rectangle) *RGBA16 {
	return &RGBA16{
		Pix:    cpu.MakePaddedSliceAligned[byte](r.Dx()*r.Dy()*2, Alignment),
		Stride: 2 * r.Dx(),
		Rect:   r,
	}
}

type colorRGBA16 uint16

func (c colorRGBA16) RGBA() (r, g, b, a uint32) {
	return uint32(c & 0xf800), uint32(c<<5) & 0xf800,
		uint32(c<<10) & 0xf800, uint32(c&1) * 0xffff
}

var RGBA16Model color.Model = color.ModelFunc(rgba16Model)

func rgba16Model(c color.Color) color.Color {
	if _, ok := c.(colorRGBA16); ok {
		return c
	}
	r, g, b, a := c.RGBA()
	return colorRGBA16((r & 0xf800) | (g&0xf800)>>5 | (b&0xf800)>>10 | a>>15)
}

func (p *RGBA16) ColorModel() color.Model { return RGBA16Model }

func (p *RGBA16) Bounds() image.Rectangle {
	return p.Rect
}

func (p *RGBA16) At(x, y int) color.Color {
	if !(image.Point{x, y}.In(p.Rect)) {
		return color.RGBA{}
	}
	offset := p.PixOffset(x, y)
	return colorRGBA16(uint16(p.Pix[offset])<<8 | uint16(p.Pix[offset+1]))
}

func (p *RGBA16) Set(x, y int, c color.Color) {
	if !(image.Point{x, y}.In(p.Rect)) {
		return
	}
	offset := p.PixOffset(x, y)
	col, _ := rgba16Model(c).(colorRGBA16)
	p.Pix[offset] = uint8(col >> 8)
	p.Pix[offset+1] = uint8(col & 0xff)
}

func (p *RGBA16) PixOffset(x, y int) int {
	return (y-p.Rect.Min.Y)*p.Stride + (x-p.Rect.Min.X)*2
}
