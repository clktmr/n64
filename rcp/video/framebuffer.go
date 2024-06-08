package video

import (
	"image"
	"image/color"
	"image/draw"
	"n64/rcp/cpu"
	"unsafe"
)

const (
	// TODO support other resolutions
	WIDTH  = 320
	HEIGHT = 240
)

// Represents an image that the DAC can read and output on a screen.
type Framebuffer interface {
	Addr() uintptr
}

type Drawer interface {
	Draw(r image.Rectangle, src image.Image, sp image.Point,
		mask image.Image, mp image.Point, op draw.Op)
	Flush()
	Bounds() image.Rectangle
}

const Alignment = 64

// Stores pixels in RGBA with 32bit (8:8:8:8)
//
// draw.DrawMask will chose optimized implementations based on type assertions.
// Thats why it's important to be an image.RGBA specifically, a type that the
// image/draw package knows.  Still all rendering done this way is without
// hardware acceleration and rather slow.
type RGBA32 struct {
	image.RGBA
}

func NewRGBA32(r image.Rectangle) *RGBA32 {
	return &RGBA32{image.RGBA{
		Pix:    cpu.MakePaddedSliceAligned(r.Dx()*r.Dy()*4, Alignment),
		Stride: 4 * r.Dx(),
		Rect:   r,
	}}
}

func (p *RGBA32) Draw(r image.Rectangle, src image.Image, sp image.Point,
	mask image.Image, mp image.Point, op draw.Op) {
	// TODO optimize drawing glyphs via type assertion
	// text: src *image.Uniform, mask *images.ImmAlphaN
	draw.DrawMask(&p.RGBA, r, src, sp, mask, mp, op)
}

func (p *RGBA32) Flush() {
	cpu.Writeback(p.Addr(), p.Stride*p.Bounds().Dy())
}

func (p *RGBA32) Addr() uintptr {
	return uintptr(unsafe.Pointer(unsafe.SliceData(p.Pix)))
}

func (p *RGBA32) SubImage(r image.Rectangle) image.Image {
	subImg, _ := p.RGBA.SubImage(r).(*image.RGBA)
	return &RGBA32{*subImg}
}

// Stores pixels in RGBA with 32bit (8:8:8:8)
//
// Same as RGBA32, but not premultiplied-alpha.
type NRGBA32 struct {
	image.NRGBA
}

func NewNRGBA32(r image.Rectangle) *NRGBA32 {
	return &NRGBA32{image.NRGBA{
		Pix:    cpu.MakePaddedSliceAligned(r.Dx()*r.Dy()*4, Alignment),
		Stride: 4 * r.Dx(),
		Rect:   r,
	}}
}

func (p *NRGBA32) Draw(r image.Rectangle, src image.Image, sp image.Point,
	mask image.Image, mp image.Point, op draw.Op) {
	// TODO optimize drawing glyphs via type assertion
	// text: src *image.Uniform, mask *images.ImmAlphaN
	draw.DrawMask(&p.NRGBA, r, src, sp, mask, mp, op)
}

func (p *NRGBA32) Flush() {
	cpu.Writeback(p.Addr(), p.Stride*p.Bounds().Dy())
}

func (p *NRGBA32) Addr() uintptr {
	return uintptr(unsafe.Pointer(unsafe.SliceData(p.Pix)))
}

func (p *NRGBA32) SubImage(r image.Rectangle) image.Image {
	subImg, _ := p.NRGBA.SubImage(r).(*image.NRGBA)
	return &NRGBA32{*subImg}
}

// Stores pixels in RGBA with 16bit (5:5:5:1)
//
// Implements draw.Image, so all the drawing tools from the standard library can
// be used.  It's slower than RGBA32 though, because there are no optimiztions
// for this type in the image/draw package.
type RGBA16 struct {
	Pix    []uint8
	Stride int
	Rect   image.Rectangle
}

func NewRGBA16(r image.Rectangle) *RGBA16 {
	return &RGBA16{
		Pix:    cpu.MakePaddedSliceAligned(r.Dx()*r.Dy()*2, Alignment),
		Stride: 2 * r.Dx(),
		Rect:   r,
	}
}

func (p *RGBA16) Draw(r image.Rectangle, src image.Image, sp image.Point,
	mask image.Image, mp image.Point, op draw.Op) {
	// TODO write optimized version instead of calling draw.DrawMask
	draw.DrawMask(p, r, src, sp, mask, mp, op)
}

func (p *RGBA16) Flush() {
	cpu.Writeback(p.Addr(), p.Stride*p.Bounds().Dy())
}

func (p *RGBA16) Addr() uintptr {
	return uintptr(unsafe.Pointer(unsafe.SliceData(p.Pix)))
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
