package gomono12

import (
	_ "embed"
	"image"
	_ "image/png"
	"strings"
)

type fontData struct {
	positions []byte
	fontMap   *image.Alpha
}

func (p *fontData) Advance(i int) int {
	return int(p.positions[3*i+2])
}

func (p *fontData) Glyph(i int) (img image.Image, origin image.Point, advance int) {
	base := 3 * i
	advance = int(p.positions[base+2])
	origin = image.Point{
		int(p.positions[base]), int(p.positions[base+1]),
	}
	rect := image.Rect(origin.X, origin.Y-Ascent, origin.X+advance, origin.Y+Height-Ascent)
	img = p.fontMap.SubImage(rect)
	return
}

//go:embed 0000_00ff.pos
var X0000_pos string

//go:embed 0000_00ff.png
var X0000_png string

func load() *fontData {
	f := &fontData{}
	f.positions = make([]byte, len(X0000_pos))
	copy(f.positions, []byte(X0000_pos))

	fontMapReader := strings.NewReader(X0000_png)
	fontMap, _, _ := image.Decode(fontMapReader)
	imgGray, _ := fontMap.(*image.Gray)
	f.fontMap = &image.Alpha{
		Pix:    imgGray.Pix,
		Stride: imgGray.Stride,
		Rect:   imgGray.Rect,
	}

	return f
}
