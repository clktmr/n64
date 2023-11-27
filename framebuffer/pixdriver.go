package framebuffer

import (
	"image"
	"image/color"
	"image/draw"
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
	// TODO cache writeback
}

func (fb *Framebuffer) Err(clear bool) error {
	return nil
}
