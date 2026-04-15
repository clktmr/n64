package draw

import (
	"image"
	"image/color"
	"image/draw"
	"slices"

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
}

var _ image.Image = &TextImage{}

func NewTextImage(f *fonts.Face, width int, fg, bg color.Color) *TextImage {
	return &TextImage{
		font:  f,
		width: width,
		dot:   image.Pt(0, int(f.Ascent)),
		cmds:  make([]cmd, 0),
		fg:    color.NRGBAModel.Convert(fg).(color.NRGBA),
		bg:    color.NRGBAModel.Convert(bg).(color.NRGBA),
	}
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

func (p *TextImage) WriteString(s string) {
	for _, r := range s {
		if r == '\n' {
			p.dot.X = 0
			p.dot.Y += int(p.font.Height)
			continue
		}
		tex, glyph := p.font.GlyphMap(r)
		if tex == nil {
			continue
		}

		var glyphRect, drawRect image.Rectangle
		var sp image.Point
		if p.bg.A != 0x0 {
			glyphRect = image.Rect(0, int(p.font.Height-p.font.Ascent), int(glyph.Advance), int(-p.font.Ascent))
			drawRect = glyphRect.Add(p.dot)
			sp = glyphRect.Min.Add(glyph.Origin.Pt())
		} else {
			glyphRect = glyph.Rect.Rect()
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

		p.dot.X += int(glyph.Advance)
	}
}

func (p *TextImage) Optimize() {
	slices.SortFunc(p.cmds, func(a, b cmd) int {
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

func (p *TextImage) Draw(dst *texture.Texture, r image.Rectangle, sp image.Point, op draw.Op) {
	rdp.RDP.SetPrimitiveColor(p.fg)

	var blendmode rdp.BlendMode
	mode := rdp.ForceBlend | rdp.BiLerp0
	if op == draw.Over {
		mode |= rdp.ImageRead
		blendmode = blendOver()
	} else {
		rdp.RDP.SetBlendColor(p.bg)
		blendmode = blendOverEnv()
	}
	rdp.RDP.SetOtherModes(rdp.OtherModes(mode,
		rdp.CycleTypeOne, rdp.RGBDitherNone, rdp.AlphaDitherNone, rdp.ZmodeOpaque, rdp.CvgDestClamp, blendmode,
	))

	rdp.RDP.SetCombineMode(rdp.CombineMode1Cycle(
		0, 0, 0, rdp.CombinePrimitive,
		0, 0, 0, rdp.CombineTex0,
	))

	tex, _ := p.font.GlyphMap(0)
	if tex == nil {
		return
	}
	loadIdx, drawIdx := rdp.RDP.SetTile(rdp.TileDescriptor{
		Format: tex.Format(),
		Addr:   0x0,
		Line:   uint16(tex.Format().TMEMWords(tex.Bounds().Dx())),
	})

	pos := r.Min.Sub(sp)
	scissor := r.Intersect(dst.Bounds())
	rdp.RDP.SetScissor(scissor, rdp.InterlaceNone)

	lasttex, lasttile := -1, -1
	for _, cmd := range p.cmds {
		if int(cmd.tex) != lasttex {
			rdp.RDP.SetTextureImage(p.textures[cmd.tex])
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
