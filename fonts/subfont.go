package fonts

import (
	"bytes"
	"image"
	"image/png"
	"path"
	"strconv"
	"strings"

	"github.com/clktmr/n64/debug"
	"github.com/clktmr/n64/drivers/cartfs"
	"github.com/clktmr/n64/rcp/texture"
	"github.com/embeddedgo/display/font/subfont"
)

// SubfontData implements [subfont.Data].
type SubfontData struct {
	height, ascent int
	positions      []byte
	fontMap        *texture.Texture
	glyphs         [256]struct { // TODO constant
		img     *texture.Texture
		origin  image.Point
		advance int
	}
}

func (p *SubfontData) Advance(i int) int {
	return int(p.positions[3*i+2])
}

func (p *SubfontData) Glyph(i int) (img image.Image, origin image.Point, advance int) {
	g := &p.glyphs[i]
	img, origin, advance = g.img, g.origin, g.advance
	return
}

func (p *SubfontData) GlyphMap(i int) (img image.Image, r image.Rectangle, origin image.Point, advance int) {
	g := &p.glyphs[i]
	img, r, origin, advance = p.fontMap, g.img.Bounds(), g.origin, g.advance
	return
}

func (p *SubfontData) glyph(i int) (img *texture.Texture, origin image.Point, advance int) {
	base := 3 * i
	advance = int(p.positions[base+2])
	origin = image.Point{
		int(p.positions[base]), int(p.positions[base+1]),
	}
	rect := image.Rect(origin.X, origin.Y-p.ascent, origin.X+advance, origin.Y+p.height-p.ascent)
	img = p.fontMap.SubImage(rect)
	return
}

// Returns data for a subfont from an image.
func NewSubfontData(pos, imgPng []byte, height, ascent int) *SubfontData {
	f := &SubfontData{
		height:    height,
		ascent:    ascent,
		positions: pos,
	}

	// TODO Store images raw instead of compressed
	fontMapReader := bytes.NewReader(imgPng)
	fontMap, err := png.Decode(fontMapReader)
	debug.AssertErrNil(err)
	imgGray, ok := fontMap.(*image.Gray)
	debug.Assert(ok, "fontmap format")
	f.fontMap = texture.NewTextureFromImage(&image.Alpha{
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

type Loader struct {
	FS             cartfs.FS
	Height, Ascent int
}

func (l Loader) Load(r rune, current []*subfont.Subfont) (containing *subfont.Subfont, updated []*subfont.Subfont) {
	entries, err := l.FS.ReadDir(".")
	if err != nil {
		panic(err)
	}
	for _, entry := range entries {
		if ext := path.Ext(entry.Name()); ext == ".pos" {
			name := strings.TrimSuffix(entry.Name(), ext)
			start, err := strconv.ParseUint(name[:4], 16, 0)
			if err != nil {
				panic(err)
			}
			end, err := strconv.ParseUint(name[5:9], 16, 0)
			if err != nil {
				panic(err)
			}
			if r >= rune(start) && r <= rune(end) {
				containing = l.loadSubfont(name, rune(start), rune(end))
				updated = append(current, containing)
				return
			}
		}
	}
	return
}

func (l Loader) loadSubfont(name string, first, last rune) *subfont.Subfont {
	sfPos, err := l.FS.ReadFile(name + ".pos")
	if err != nil {
		panic(err)
	}
	sfPng, err := l.FS.ReadFile(name + ".png")
	if err != nil {
		panic(err)
	}
	return &subfont.Subfont{
		First:  first,
		Last:   last,
		Offset: 0,
		Data:   NewSubfontData(sfPos, sfPng, l.Height, l.Ascent),
	}
}
