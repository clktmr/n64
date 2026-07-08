package draw

import (
	"image"
	"image/color"
	"image/draw"
	"slices"
	"unicode"

	"github.com/clktmr/n64/fonts"
	"github.com/clktmr/n64/rcp/fixed"
	"github.com/clktmr/n64/rcp/rdp"
	"github.com/clktmr/n64/rcp/texture"
)

type cmd struct {
	tile, r fixed.RectangleU10_2
	sp      fixed.Point11_5
	tex     uint32
}

type TextImage struct {
	font          *fonts.Face
	width, height int
	dot           image.Point
	cmds          []cmd
	textures      []*texture.Texture
	fg, bg        color.NRGBA

	lastSpace      int
	curTokenOrigin image.Point

	Wrap int
}

var _ image.Image = &TextImage{}

func NewTextImage(f *fonts.Face, fg, bg color.Color) *TextImage {
	p := &TextImage{
		font: f,
		Wrap: int(^uint(0) >> 1),
		cmds: make([]cmd, 0),
		fg:   color.NRGBAModel.Convert(fg).(color.NRGBA),
		bg:   color.NRGBAModel.Convert(bg).(color.NRGBA),
	}
	p.Reset()
	return p
}

func (p *TextImage) ColorModel() color.Model {
	tex, _ := p.font.GlyphMap(0)
	return tex.ColorModel()
}

func (p *TextImage) Bounds() image.Rectangle {
	return image.Rect(0, 0, p.width, p.height)
}

func (p *TextImage) At(x, y int) color.Color {
	return color.Black // TODO
}

// Reset removes all text from the image.
func (p *TextImage) Reset() {
	p.cmds = p.cmds[:0]
	p.width = 0
	p.height = int(p.font.Height)
	p.dot = image.Pt(0, int(p.font.Ascent))
	p.curTokenOrigin = p.dot
	p.lastSpace = -1
}

// WriteString appends s to the TextImage.
// Line feeds (\n) are represented as newline.
func (p *TextImage) WriteString(s string) {
	for _, r := range s {
		lastGlyph := len(p.cmds)

		if unicode.IsSpace(r) {
			p.sortCommands(lastGlyph, p.lastSpace)
			p.lastSpace = lastGlyph
		}
		if r == '\n' {
			p.newline()
			continue
		}
		if lastGlyph == p.lastSpace+1 { // first rune of word
			p.curTokenOrigin = p.dot
		}

		tex, glyph := p.font.GlyphMap(r)
		if tex == nil {
			continue
		}

		// Check if we need to wrap
		if p.dot.X+int(glyph.Rect.Max.X-glyph.Origin.X) > p.Wrap &&
			p.curTokenOrigin.X != 0 {
			if p.lastSpace == lastGlyph { // wrap caused by whitespace
				p.newline()
				continue
			}
			// walk over the last token and move it to the newline
			curToken := p.cmds[p.lastSpace+1:]
			curTokenAdv := p.dot.X - p.curTokenOrigin.X
			p.newline()
			trans := p.curTokenOrigin.Sub(p.dot)
			for i := range curToken {
				curToken[i].r = fixed.RectU10_2R(curToken[i].r.Rect().Sub(trans))
			}
			p.dot.X += curTokenAdv
		}

		var glyphRect, drawRect image.Rectangle
		var sp image.Point
		if p.bg.A != 0x0 {
			glyphRect = image.Rect(0, int(p.font.Height-p.font.Ascent), int(glyph.Advance), int(-p.font.Ascent))
			drawRect = glyphRect.Add(p.dot)
			sp = glyphRect.Min.Add(glyph.Origin.Pt())
		} else {
			glyphRect = glyph.Rect.Rect()
			// FIXME drawRect.Min might be negative and overflow in
			// later RectU10_2 conversion.
			drawRect = glyphRect.Add(p.dot.Sub(glyph.Origin.Pt()))
			sp = glyphRect.Min
		}

		texIdx := slices.Index(p.textures, tex)
		if texIdx == -1 {
			texIdx = len(p.textures)
			p.textures = append(p.textures, tex)
		}
		p.cmds = append(p.cmds, cmd{
			tile: fixed.RectU10_2R(glyph.Rect.Rect()),
			r:    fixed.RectU10_2R(drawRect),
			sp:   fixed.Pt11_5P(sp),
			tex:  uint32(texIdx),
		})

		p.width = max(p.width, p.dot.X+int(glyph.Rect.Max.X-glyph.Origin.X))
		p.dot.X += int(glyph.Advance)
	}
}

// insertionSortCmpFunc sorts data[:] using insertion sort, assuming data is
// already sorted up until n.
func insertionSortCmpFunc[E any](data []E, n int, cmp func(a, b E) int) {
	for i := n; i < len(data); i++ {
		for j := i; j > 0 && cmp(data[j], data[j-1]) < 0; j-- {
			data[j], data[j-1] = data[j-1], data[j]
		}
	}
}

// sortCommands sorts the internal displaylist to minimize tile loading.
func (p *TextImage) sortCommands(until, sorted int) {
	// Only sort up to last space, in case we need to wrap the current token
	// in a later WriteString call.
	insertionSortCmpFunc(p.cmds[:until], sorted, func(a, b cmd) int {
		if a.tex > b.tex {
			return 1
		} else if a.tex < b.tex {
			return -1
		}
		if a.tile > b.tile {
			return 1
		} else if a.tile < b.tile {
			return -1
		}
		return 0
	})
}

func (p *TextImage) newline() {
	p.dot.X = 0
	p.dot.Y += int(p.font.Height)
	p.height += int(p.font.Height)
}

func drawTextImage(dst *texture.Texture, r image.Rectangle, src *TextImage, sp image.Point, op draw.Op) {
	rdp.RDP.SetPrimitiveColor(src.fg)

	var blendmode rdp.BlendMode
	mode := rdp.ForceBlend | rdp.BiLerp0
	if op == draw.Over {
		mode |= rdp.ImageRead
		blendmode = blendOver()
	} else {
		rdp.RDP.SetBlendColor(src.bg)
		blendmode = blendOverEnv()
	}
	rdp.RDP.SetOtherModes(rdp.OtherModes(mode,
		rdp.CycleTypeOne, rdp.RGBDitherNone, rdp.AlphaDitherNone, rdp.ZmodeOpaque, rdp.CvgDestClamp, blendmode,
	))

	rdp.RDP.SetCombineMode(rdp.CombineMode1Cycle(
		0, 0, 0, rdp.CombinePrimitive,
		0, 0, 0, rdp.CombineTex0,
	))

	tex, _ := src.font.GlyphMap(0)
	if tex == nil {
		return
	}
	loadIdx, drawIdx := rdp.RDP.SetTile(rdp.TileDescriptor{
		Format: tex.Format(),
		Addr:   0x0,
		Line:   uint16(tex.Format().TMEMWords(tex.Bounds().Dx())),
	})

	pos := r.Min.Sub(sp)
	rdp.RDP.SetScissor(r, rdp.InterlaceNone)

	lasttex, lasttile := -1, -1
	for _, cmd := range src.cmds {
		if int(cmd.tex) != lasttex {
			rdp.RDP.SetTextureImage(src.textures[cmd.tex])
			lasttex = int(cmd.tex)
		}
		if int(cmd.tile) != lasttile {
			rdp.RDP.LoadTile(loadIdx, cmd.tile.Rect())
			lasttile = int(cmd.tile)
		}
		r := cmd.r.Rect().Add(image.Pt(pos.X, pos.Y))
		rdp.RDP.TextureRectangle(r, cmd.sp.Pt(), image.Pt(1, 1), drawIdx)
	}
}
