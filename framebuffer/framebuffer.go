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

var Buf = [WIDTH * HEIGHT * 4]uint8{}

// Represents an image that the DAC can read and output on a screen. Implements
// draw.Image, so all the drawing tools from the standard library can be used.
// Moreover draw.DrawMask will chose optimized implementations based on type
// assertions.  Thats why it's important to be a image.RGBA specifically,  a
// type that the draw package knows. Still all rendering done this way is
// withoug hardware acceleration and rather slow.
// TODO support 16 bit per pixel, other resolutions and double buffering
type Framebuffer struct {
	image.RGBA
	fill image.Uniform
}

func NewFramebuffer() *Framebuffer {
	fb := &Framebuffer{
		RGBA: image.RGBA{
			Pix:    Buf[:],
			Stride: WIDTH * 4,
			Rect:   image.Rect(0, 0, WIDTH, HEIGHT),
		},
	}
	return fb
}

func (fb *Framebuffer) Draw(r image.Rectangle, src image.Image, sp image.Point,
	mask image.Image, mp image.Point, op draw.Op) {
	// TODO optimize drawing glyphs via type assertion
	// text: *image.Uniform, mask *images.ImmAlphaN
	draw.DrawMask(fb, r, src, sp, mask, mp, op)
}

func (fb *Framebuffer) Fill(rect image.Rectangle) {
	r, g, b, a := fb.fill.C.RGBA()
	for line := rect.Min.Y; line < rect.Max.Y; line++ {
		for row := rect.Min.X; row < rect.Max.X; row++ {
			base := (line<<2)*fb.Rect.Dx() + (row << 2)
			fb.Pix[base] = uint8(r)
			fb.Pix[base+1] = uint8(g)
			fb.Pix[base+2] = uint8(b)
			fb.Pix[base+3] = uint8(a)
		}
	}
}

func (fb *Framebuffer) SetColor(c color.Color) {
	fb.fill.C = c
}

func (fb *Framebuffer) SetDir(dir int) image.Rectangle {
	return fb.Bounds()
}

func (fb *Framebuffer) Flush() {}

func (fb *Framebuffer) Err(clear bool) error {
	return nil
}
