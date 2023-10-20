package framebuffer

import (
	"image"
	"image/color"
	"image/draw"
)

const (
	WIDTH  = 320
	HEIGHT = 240
)

var Buf = [WIDTH * HEIGHT]uint32{}

// Represents an image that the DAC can read and output on a screen. Implements
// draw.Image, so all the drawing tools from the standard library can be used.
// But be aware that all rendering done this way is withoug hardware
// acceleration and currently not optimized.
// TODO support 16 bit per pixel, other resolutions and double buffering
type Framebuffer struct {
	buf           []uint32
	height, width int
	fill          uint32
}

func NewFramebuffer() *Framebuffer {
	fb := Framebuffer{
		buf:    Buf[:],
		width:  WIDTH,
		height: HEIGHT,
	}
	return &fb
}

func (fb *Framebuffer) ColorModel() color.Model {
	return color.RGBAModel
}

func (fb *Framebuffer) Bounds() image.Rectangle {
	return image.Rect(0, 0, WIDTH, HEIGHT)
}

func (fb *Framebuffer) At(x, y int) color.Color {
	c := fb.buf[y*fb.width+x]
	return color.RGBA{
		R: uint8(c >> 24),
		G: uint8(c >> 16),
		B: uint8(c >> 8),
		A: uint8(0),
	}
}

func (fb *Framebuffer) Set(x, y int, c color.Color) {
	r, g, b, a := c.RGBA()
	fb.buf[y*fb.width+x] = ((r & 0xff00) << 16) | ((g & 0xff00) << 8) |
		(b & 0xff00) | ((a & 0xff00) >> 8)
}

func (fb *Framebuffer) Draw(r image.Rectangle, src image.Image, sp image.Point,
	mask image.Image, mp image.Point, op draw.Op) {
	draw.DrawMask(fb, r, src, sp, mask, mp, op)
}

func (fb *Framebuffer) Fill(r image.Rectangle) {
	for line := r.Min.Y; line < r.Max.Y; line++ {
		for row := r.Min.X; row < r.Max.X; row++ {
			fb.buf[line*fb.width+row] = fb.fill
		}
	}
}

func (fb *Framebuffer) SetColor(c color.Color) {
	r, g, b, a := c.RGBA()
	fb.fill = ((r & 0xff00) << 16) | ((g & 0xff00) << 8) |
		(b & 0xff00) | ((a & 0xff00) >> 8)
}

func (fb *Framebuffer) SetDir(dir int) image.Rectangle {
	return image.Rect(0, 0, fb.width, fb.height)
}

func (fb *Framebuffer) Flush() {}

func (fb *Framebuffer) Err(clear bool) error {
	return nil
}
