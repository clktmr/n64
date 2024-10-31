package gomono12

import (
	_ "embed"
	"image"
	"image/png"
	"strings"

	"github.com/clktmr/n64/rcp/texture"
)

type fontData struct {
	positions []byte
	fontMap   *texture.I8
	glyphs    [256]struct {
		img     texture.I8
		origin  image.Point
		advance int
	}
}

func (p *fontData) Advance(i int) int {
	return int(p.positions[3*i+2])
}

func (p *fontData) Glyph(i int) (img image.Image, origin image.Point, advance int) {
	g := &p.glyphs[i]
	img, origin, advance = &g.img, g.origin, g.advance
	return
}

func (p *fontData) glyph(i int) (img texture.I8, origin image.Point, advance int) {
	base := 3 * i
	advance = int(p.positions[base+2])
	origin = image.Point{
		int(p.positions[base]), int(p.positions[base+1]),
	}
	rect := image.Rect(origin.X, origin.Y-Ascent, origin.X+advance, origin.Y+Height-Ascent)
	img = *p.fontMap.SubImage(rect)
	return
}

//go:embed 0000_00ff.pos
var X0000_pos string

//go:embed 0000_00ff.png
var X0000_png string

func load() *fontData {
	f := &fontData{}
	f.positions = []byte(X0000_pos)

	fontMapReader := strings.NewReader(X0000_png)
	fontMap, _ := png.Decode(fontMapReader)
	imgGray, _ := fontMap.(*image.Gray)
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
