package draw

import (
	"image"
	"image/draw"

	"github.com/clktmr/n64/rcp/texture"
)

// SW implements a software-based drawer by forwarding the calls to image.draw.
//
// It ensures to that image.draw uses the optimized path by passing images
// instead of textures. Note that as of now using 32bpp textures has better
// performance, since there is no optimized implementation for 16bpp images.
type SW draw.Op

const (
	OverSW = SW(draw.Over)
	SrcSW  = SW(draw.Src)
)

func (op SW) Draw(dst draw.Image, r image.Rectangle, src image.Image, sp image.Point) {
	op.DrawMask(dst, r, src, sp, nil, image.Point{})
}

func (op SW) DrawMask(dst draw.Image, r image.Rectangle, src image.Image, sp image.Point, mask image.Image, mp image.Point) {
	if tex, ok := src.(texture.Texture); ok {
		src = tex.Image
	}
	if tex, ok := mask.(texture.Texture); ok {
		mask = tex.Image
	}
	draw.DrawMask(dst, r, src, sp, mask, mp, draw.Op(op))
}
