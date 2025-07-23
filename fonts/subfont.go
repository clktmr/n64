package fonts

import (
	"bytes"
	"encoding/binary"
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
	fontMap *texture.Texture
	glyphs  []Glyph
}

func (p *SubfontData) Advance(i int) int {
	return int(p.glyphs[i].Advance)
}

func (p *SubfontData) Glyph(i int) (img image.Image, origin image.Point, advance int) {
	g := &p.glyphs[i]
	r := image.Rect(int(g.Rect.Min.X), int(g.Rect.Min.Y), int(g.Rect.Max.X), int(g.Rect.Max.Y))
	img = p.fontMap.SubImage(r)
	origin = image.Pt(int(g.Origin.X), int(g.Origin.Y))
	advance = int(g.Advance)
	return
}

//go:nosplit
func (p *SubfontData) GlyphMap(i int) (img image.Image, r image.Rectangle, origin image.Point, advance int) {
	g := &p.glyphs[i]
	img = p.fontMap
	r = image.Rect(int(g.Rect.Min.X), int(g.Rect.Min.Y), int(g.Rect.Max.X), int(g.Rect.Max.Y))
	origin = image.Pt(int(g.Origin.X), int(g.Origin.Y))
	advance = int(g.Advance)
	return
}

// Returns data for a subfont from an image.
func NewSubfontData(pos, tex []byte) *SubfontData {
	f := &SubfontData{}

	fontMap, err := texture.Load(bytes.NewReader(tex))
	if err != nil {
		panic(err)
	}
	f.fontMap = fontMap

	// TODO perf: use unsafe to cast pos from []byte to []Glyph
	f.glyphs = make([]Glyph, len(pos)/7)
	err = binary.Read(bytes.NewReader(pos), binary.BigEndian, &f.glyphs)
	if err != nil {
		panic(err)
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
		Data:   NewSubfontData(sfPos, sfTex),
	}
}
