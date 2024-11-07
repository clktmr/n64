package fonts

import (
	"image"
	"image/png"
	"strings"

	"github.com/clktmr/n64/debug"
	"github.com/clktmr/n64/rcp/texture"
)

type SubfontData struct {
	height, ascent int
	positions      []byte
	fontMap        *texture.I8
	glyphs         [256]struct { // TODO constant
		img     texture.I8
		origin  image.Point
		advance int
	}
}

func (p *SubfontData) Advance(i int) int {
	return int(p.positions[3*i+2])
}

func (p *SubfontData) Glyph(i int) (img image.Image, origin image.Point, advance int) {
	g := &p.glyphs[i]
	img, origin, advance = &g.img, g.origin, g.advance
	return
}

func (p *SubfontData) GlyphMap(i int) (img image.Image, r image.Rectangle, origin image.Point, advance int) {
	g := &p.glyphs[i]
	img, r, origin, advance = p.fontMap, g.img.Rect, g.origin, g.advance
	return
}

func (p *SubfontData) glyph(i int) (img texture.I8, origin image.Point, advance int) {
	base := 3 * i
	advance = int(p.positions[base+2])
	origin = image.Point{
		int(p.positions[base]), int(p.positions[base+1]),
	}
	rect := image.Rect(origin.X, origin.Y-p.ascent, origin.X+advance, origin.Y+p.height-p.ascent)
	img = *p.fontMap.SubImage(rect)
	return
}

func NewSubfontData(pos, imgPng string, height, ascent int) *SubfontData {
	f := &SubfontData{
		height:    height,
		ascent:    ascent,
		positions: []byte(pos),
	}

	fontMapReader := strings.NewReader(imgPng)
	fontMap, err := png.Decode(fontMapReader)
	debug.AssertErrNil(err)
	imgGray, ok := fontMap.(*image.Gray)
	debug.Assert(ok, "fontmap format")
	f.fontMap = texture.NewI8FromImage(&image.Alpha{
		Pix:    imgGray.Pix,
		Stride: imgGray.Stride,
		Rect:   imgGray.Rect,
	})

	for i := range f.glyphs {
		g := &f.glyphs[i]
		g.img, g.origin, g.advance = f.glyph(i)
	}

	return f
}
