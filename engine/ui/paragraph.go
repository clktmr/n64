package ui

import (
	"image"
	idraw "image/draw"

	"github.com/clktmr/n64/drivers/draw"
)

type UIParagraph struct {
	Node

	Clip bool // Clip words longer than wrap length

	img  *draw.TextImage
	text string

	sp image.Point
}

func NewUIParagraph(parent Node, img *draw.TextImage, text string) *UIParagraph {
	node := &UIParagraph{
		img:  img,
		text: text,
	}
	node.Node = parent.AddChild(node)
	return node
}

func (p *UIParagraph) Measure(width int) (dx, dy int) {
	// TODO Is it ok to modify the image in Measure?
	p.img.Wrap = width
	p.img.Reset()
	p.img.WriteString(p.text)
	if p.Clip {
		bounds := p.img.Bounds()
		dx, dy = width, bounds.Dy()
	} else {
		if p.img.Bounds().Dx() > width {
			p.img.Wrap = p.img.Bounds().Dx()
			p.img.Reset()
			p.img.WriteString(p.text)
		}
		bounds := p.img.Bounds()
		dx, dy = bounds.Dx(), bounds.Dy()
	}
	return
}

func (p *UIParagraph) Layout(r image.Rectangle, sp image.Point) {
	// p.img.Wrap = r.Dx() - sp.X
	// p.img.Reset()
	// p.img.WriteString(p.text)

	p.SetBounds(r)
	p.sp = sp
}

func (p *UIParagraph) Render(dst idraw.Image, r image.Rectangle, sp image.Point) {
	draw.Over.Draw(dst, r, p.img, p.sp.Add(sp))
}

func (p *UIParagraph) Focus(d FocusDirection) bool { return false }
func (p *UIParagraph) Focused() int                { return 0 }
