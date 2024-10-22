package texture

import (
	"image"
	"image/color"
)

type imageRGBA16 struct {
	Pix    []uint8
	Stride int
	Rect   image.Rectangle
}

func (p *imageRGBA16) ColorModel() color.Model { return RGBA16Model }

func (p *imageRGBA16) Bounds() image.Rectangle {
	return p.Rect
}

func (p *imageRGBA16) At(x, y int) color.Color {
	if !(image.Point{x, y}.In(p.Rect)) {
		return color.RGBA{}
	}
	offset := p.PixOffset(x, y)
	return colorRGBA16(uint16(p.Pix[offset])<<8 | uint16(p.Pix[offset+1]))
}

func (p *imageRGBA16) Set(x, y int, c color.Color) {
	if !(image.Point{x, y}.In(p.Rect)) {
		return
	}
	offset := p.PixOffset(x, y)
	col, _ := rgba16Model(c).(colorRGBA16)
	p.Pix[offset] = uint8(col >> 8)
	p.Pix[offset+1] = uint8(col & 0xff)
}

func (p *imageRGBA16) PixOffset(x, y int) int {
	return (y-p.Rect.Min.Y)*p.Stride + (x-p.Rect.Min.X)*2
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
