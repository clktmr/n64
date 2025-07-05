package fonts

import (
	"bytes"
	"image"
	"path"
	"strconv"
	"strings"

	"github.com/clktmr/n64/drivers/cartfs"
	"github.com/clktmr/n64/rcp/texture"
	"github.com/embeddedgo/display/font/subfont"
)

// SubfontData implements [subfont.Data].
type SubfontData struct {
	height, ascent int
	positions      []byte
	fontMap        *texture.Texture
	glyphs         []glyphData
}

type glyphData struct {
	img     *texture.Texture
	origin  image.Point
	advance int
}

func (p *SubfontData) Advance(i int) int {
	return int(p.positions[7*i+6])
}

func (p *SubfontData) Glyph(i int) (img image.Image, origin image.Point, advance int) {
	g := &p.glyphs[i]
	img, origin, advance = g.img, g.origin, g.advance
	return
}

//go:nosplit
func (p *SubfontData) GlyphMap(i int) (img image.Image, r image.Rectangle, origin image.Point, advance int) {
	g := &p.glyphs[i]
	img, r, origin, advance = p.fontMap, g.img.Bounds(), g.origin, g.advance
	return
}

func (p *SubfontData) glyph(i int) (img *texture.Texture, origin image.Point, advance int) {
	base := 7 * i
	advance = int(p.positions[base+6])
	origin = image.Pt(
		int(p.positions[base+0]), int(p.positions[base+1]),
	)
	rect := image.Rect(
		int(p.positions[base+2]), int(p.positions[base+3]),
		int(p.positions[base+4]), int(p.positions[base+5]),
	)
	img = p.fontMap.SubImage(rect)
	return
}

// Returns data for a subfont from an image.
func NewSubfontData(pos, tex []byte, height, ascent int) *SubfontData {
	f := &SubfontData{
		height:    height,
		ascent:    ascent,
		positions: pos,
	}

	fontMapReader := bytes.NewReader(tex)
	fontMap, err := texture.Load(fontMapReader)
	if err != nil {
		panic(err)
	}
	f.fontMap = fontMap

	f.glyphs = make([]glyphData, len(pos)/7)
	for i := range f.glyphs {
		g := &f.glyphs[i]
		g.img, g.origin, g.advance = f.glyph(i)
	}

	return f
}

type Loader struct {
	FS             *cartfs.FS
	Height, Ascent int
	files          []subfontPath
}

type subfontPath struct {
	first, last rune
	path        string
}

func NewLoader(fs *cartfs.FS, height, ascent int) (l *Loader) {
	l = &Loader{fs, height, ascent, nil}

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
			l.files = append(l.files, subfontPath{rune(start), rune(end), name})
		}
	}
	return
}

func (l *Loader) Load(r rune, current []*subfont.Subfont) (containing *subfont.Subfont, updated []*subfont.Subfont) {
	for _, f := range l.files {
		if r >= f.first && r <= f.last {
			containing = l.loadSubfont(f.path, f.first, f.last)
			updated = append(current, containing)
			return
		}
	}
	updated = current
	return
}

func (l *Loader) loadSubfont(name string, first, last rune) *subfont.Subfont {
	sfPos, err := l.FS.ReadFile(name + ".pos")
	if err != nil {
		panic(err)
	}
	sfTex, err := l.FS.ReadFile(name + ".tex")
	if err != nil {
		panic(err)
	}
	return &subfont.Subfont{
		First:  first,
		Last:   last,
		Offset: 0,
		Data:   NewSubfontData(sfPos, sfTex, l.Height, l.Ascent),
	}
}
