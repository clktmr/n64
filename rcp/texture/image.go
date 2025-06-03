package texture

import (
	"errors"
	"image"
	"image/color"

	"github.com/clktmr/n64/rcp/cpu"
)

// FIXME these need SubImage implementations

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
		return colorRGBA16(0)
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

type imageI4 struct {
	Pix    []uint8
	Stride int
	Rect   image.Rectangle
}

func (p *imageI4) ColorModel() color.Model { return I4Model }

func (p *imageI4) Bounds() image.Rectangle {
	return p.Rect
}

func (p *imageI4) At(x, y int) color.Color {
	if !(image.Point{x, y}.In(p.Rect)) {
		return color.RGBA{}
	}
	offset := p.PixOffset(x, y)
	if x&0x1 == 0 {
		return colorI4(p.Pix[offset] >> 4)
	} else {
		return colorI4(p.Pix[offset] &^ 0xf0)
	}
}

func (p *imageI4) Set(x, y int, c color.Color) {
	if !(image.Point{x, y}.In(p.Rect)) {
		return
	}
	offset := p.PixOffset(x, y)
	col, _ := i4Model(c).(colorI4)
	if x&0x1 == 0 {
		p.Pix[offset] &^= 0xf0
		p.Pix[offset] |= uint8(col << 4)
	} else {
		p.Pix[offset] &^= 0x0f
		p.Pix[offset] |= uint8(col)
	}
}

func (p *imageI4) PixOffset(x, y int) int {
	return (y-p.Rect.Min.Y)*p.Stride + (x-p.Rect.Min.X)/2
}

type colorI4 uint8

func (c colorI4) RGBA() (r, g, b, a uint32) {
	y := uint32(c)
	y |= y << 4
	y |= y << 8
	return y, y, y, 0xffff
}

var I4Model color.Model = color.ModelFunc(i4Model)

func i4Model(c color.Color) color.Color {
	if _, ok := c.(colorI4); ok {
		return c
	}
	r, g, b, _ := c.RGBA()

	y := (19595*r + 38470*g + 7471*b + 1<<15) >> 28

	return colorI4(y)
}

type imageCI8 struct {
	Pix     []uint8
	Stride  int
	Rect    image.Rectangle
	Palette *ColorPalette
}

func (p *imageCI8) ColorModel() color.Model { return p.Palette }

func (p *imageCI8) Bounds() image.Rectangle { return p.Rect }

func (p *imageCI8) At(x, y int) color.Color {
	if !(image.Point{x, y}.In(p.Rect)) {
		return colorRGBA16(0)
	}
	i := p.PixOffset(x, y)
	return p.Palette.At(int(p.Pix[i]), 0)
}

func (p *imageCI8) PixOffset(x, y int) int {
	return (y-p.Rect.Min.Y)*p.Stride + (x - p.Rect.Min.X)
}

func (p *imageCI8) Set(x, y int, c color.Color) {
	if !(image.Point{x, y}.In(p.Rect)) {
		return
	}
	i := p.PixOffset(x, y)
	p.Pix[i] = uint8(p.Palette.Index(c))
}

func (p *imageCI8) SubImage(r image.Rectangle) image.Image {
	r = r.Intersect(p.Rect)
	if r.Empty() {
		return &imageCI8{
			Palette: p.Palette,
		}
	}
	i := p.PixOffset(r.Min.X, r.Min.Y)
	return &imageCI8{
		Pix:     p.Pix[i:],
		Stride:  p.Stride,
		Rect:    p.Rect.Intersect(r),
		Palette: p.Palette,
	}
}

// ColorPalette is a palette of RGBA16 colors.
type ColorPalette = imageRGBA16

func CopyColorPalette(src color.Palette) (*ColorPalette, error) {
	p, err := NewColorPalette(len(src))
	if err != nil {
		return nil, err
	}

	for i, c := range src {
		p.Set(i, 0, c)
	}
	return p, nil
}

func NewColorPalette(size int) (*ColorPalette, error) {
	if size > 256 {
		return nil, errors.New("palette to large (>256)")
	}
	p := &imageRGBA16{
		Pix:    cpu.MakePaddedSliceAligned[byte](int(size)*2, alignFramebuffer),
		Stride: 2 * int(size),
		Rect:   image.Rect(0, 0, int(size), 1),
	}

	return (*ColorPalette)(p), nil
}

// Convert returns the palette color closest to c in Euclidean R,G,B space.
func (p *ColorPalette) Convert(c color.Color) color.Color {
	return p.At(p.Index(c), 0)
}

// Index returns the index of the palette color closest to c in Euclidean
// R,G,B,A space.
func (p *ColorPalette) Index(c color.Color) int {
	cr, cg, cb, ca := c.RGBA()
	ret, bestSum := 0, uint32(1<<32-1)
	for i := range p.Rect.Dx() {
		v := p.At(i, 0)
		vr, vg, vb, va := v.RGBA()
		sum := sqDiff(cr, vr) + sqDiff(cg, vg) + sqDiff(cb, vb) + sqDiff(ca, va)
		if sum < bestSum {
			if sum == 0 {
				return i
			}
			ret, bestSum = i, sum
		}
	}
	return ret
}

func sqDiff(x, y uint32) uint32 {
	d := x - y
	return (d * d) >> 2
}
