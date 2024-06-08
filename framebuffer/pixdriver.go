package framebuffer

import (
	"image"
	"image/color"
	"image/draw"
	"n64/rcp/cpu"
)

func (fb *Framebuffer) Draw(r image.Rectangle, src image.Image, sp image.Point,
	mask image.Image, mp image.Point, op draw.Op) {
	// TODO optimize drawing glyphs via type assertion
	// text: src *image.Uniform, mask *images.ImmAlphaN
	draw.DrawMask(fb.write, r, src, sp, mask, mp, op)
}

func (fb *Framebuffer) Fill(rect image.Rectangle) {
	// TODO optimize via type assertion
	fb.Draw(rect, &fb.fill, image.Point{}, nil, image.Point{}, draw.Over)
}

func (fb *Framebuffer) SetColor(c color.Color) {
	fb.fill.C = c
}

func (fb *Framebuffer) SetDir(dir int) image.Rectangle {
	return fb.read.Bounds()
}

func (fb *Framebuffer) Flush() {
	switch buf := fb.write.(type) {
	case *RGBA16:
		cpu.Writeback(fb.Addr(), buf.Stride*buf.Bounds().Dy())
	case *image.RGBA:
		cpu.Writeback(fb.Addr(), buf.Stride*buf.Bounds().Dy())
	}
}

func (fb *Framebuffer) Err(clear bool) error {
	return nil
}
