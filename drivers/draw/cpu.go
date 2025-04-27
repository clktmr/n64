package draw

import (
	"image"
	"image/draw"

	"github.com/clktmr/n64/rcp/cpu"
	"github.com/clktmr/n64/rcp/texture"
)

// A software based draw implementation.
//
// Note that as of now using 32bpp textures has better performance, since they
// use the optimized implementation from image/draw.
type Cpu struct {
	target texture.ImageTexture
}

func NewCpu() *Cpu {
	return &Cpu{}
}

func (p *Cpu) SetFramebuffer(fb texture.ImageTexture) {
	p.target = fb
}

func (fb *Cpu) Draw(r image.Rectangle, src image.Image, sp image.Point, op draw.Op) {
	fb.DrawMask(r, src, sp, nil, image.Point{}, op)
}

func (p *Cpu) DrawMask(r image.Rectangle, src image.Image, sp image.Point, mask image.Image, mp image.Point, op draw.Op) {
	if tex, ok := src.(texture.ImageTexture); ok {
		src = tex.Image()
	}
	if tex, ok := mask.(texture.ImageTexture); ok {
		mask = tex.Image()
	}
	draw.DrawMask(p.target.Image(), r, src, sp, mask, mp, op)
}

func (p *Cpu) Flush() {
	if tex, ok := p.target.(cpu.Cached); ok {
		tex.Writeback()
	}
}
