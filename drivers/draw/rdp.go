// Package draw provides hardware accelerated implementations of [draw.Drawer].
package draw

import (
	"image"
	"image/color"
	"image/draw"
	"unicode/utf8"

	"github.com/clktmr/n64/debug"
	"github.com/clktmr/n64/fonts"
	"github.com/clktmr/n64/rcp/rdp"
	"github.com/clktmr/n64/rcp/texture"

	"github.com/embeddedgo/display/images"
)

// Rdp provides hardware accelerated drawing capabilities.
//
// Besides implementing [draw.Drawer], it provides additional functions for
// drawing text efficiently.
type Rdp struct {
	target *texture.Texture
	dlist  *rdp.DisplayList
}

func NewRdp() *Rdp {
	r := &Rdp{
		dlist: &rdp.RDP,
	}

	return r
}

func (fb *Rdp) SetFramebuffer(tex *texture.Texture) {
	fb.target = tex
	fb.dlist.SetColorImage(fb.target)
	fb.dlist.SetScissor(image.Rectangle{Max: fb.target.Bounds().Size()}, rdp.InterlaceNone)
}

func (fb *Rdp) Draw(r image.Rectangle, src image.Image, sp image.Point, op draw.Op) {
	fb.DrawMask(r, src, sp, nil, image.Point{}, op)
}

func (fb *Rdp) DrawMask(r image.Rectangle, src image.Image, sp image.Point, mask image.Image, mp image.Point, op draw.Op) {
	// Readjust r if we draw to a viewport/subimage of the framebuffer
	r = r.Bounds().Sub(fb.target.Bounds().Min)

	if !r.Overlaps(fb.target.Bounds()) {
		return
	}

	switch srcImg := src.(type) {
	case *texture.Texture:
		switch mask.(type) {
		case nil:
			_, isAlpha := srcImg.Image.(*image.Alpha)
			if isAlpha {
				fb.drawColorImage(r, srcImg, sp, image.Point{1, 1}, color.RGBA{0xff, 0xff, 0xff, 0xff}, op)
			} else {
				fb.drawColorImage(r, srcImg, sp, image.Point{1, 1}, nil, op)
			}
			return
		}
	case *image.Uniform:
		switch maskImg := mask.(type) {
		case nil:
			// fill
			switch op {
			case draw.Src:
				fb.drawUniformSrc(r, srcImg.C, nil)
				return
			case draw.Over:
				fb.drawUniformOver(r, srcImg.C, color.Opaque)
				return
			}
		case *image.Uniform:
			switch op {
			case draw.Src:
				fb.drawUniformSrc(r, srcImg.C, maskImg.C)
				return
			case draw.Over:
				fb.drawUniformOver(r, srcImg.C, maskImg.C)
				return
			}
		case *texture.Texture:
			fb.drawColorImage(r, maskImg, mp, image.Point{1, 1}, srcImg.C, op)
			return
		case *images.Magnifier:
			maskAlpha, ok := maskImg.Image.(*texture.Texture)
			debug.Assert(ok, "rdp unsupported magnifier format")
			fb.drawColorImage(r, maskAlpha, mp, image.Point{maskImg.Sx, maskImg.Sy}, srcImg.C, op)
			return
		}
	}

	debug.Assert(false, "rdp unsupported format")
}

func (fb *Rdp) drawUniformSrc(r image.Rectangle, fill color.Color, mask color.Color) {
	if mask != nil {
		rf, gf, bf, af := fill.RGBA()
		_, _, _, ma := mask.RGBA()
		m := uint32(ma)
		fill = color.RGBA{
			uint8((rf * m) >> 24),
			uint8((gf * m) >> 24),
			uint8((bf * m) >> 24),
			uint8((af * m) >> 24),
		}
	}
	fb.dlist.SetFillColor(fill)
	fb.dlist.SetOtherModes(
		0, rdp.CycleTypeFill, rdp.RGBDitherNone, rdp.AlphaDitherNone, rdp.ZmodeOpaque, rdp.CvgDestClamp, rdp.BlendMode{},
	)
	fb.dlist.FillRectangle(r)
}

func (fb *Rdp) drawUniformOver(r image.Rectangle, fill color.Color, mask color.Color) {
	// CycleTypeFill doesn't support blending, use CycleTypeOne instead. The
	// following operation is required by draw.Over:
	//
	//     a = 1.0 - (fill_alpha * mask_alpha)
	//     dst = (dst*a + fill*mask_alpha)

	fb.dlist.SetPrimitiveColor(fill)
	fb.dlist.SetEnvironmentColor(mask)

	// cc = fill*mask_alpha
	cp := rdp.CombineParams{
		rdp.CombinePrimitive, rdp.CombineBColorZero,
		rdp.CombineCColorEnvironmentAlpha, rdp.CombineDColorZero,
	}
	// cc_alpha = 1-fill_alpha*mask_alpha
	cpA := rdp.CombineParams{
		rdp.CombineAAlphaZero, rdp.CombineEnvironment,
		rdp.CombinePrimitive, rdp.CombineDAlphaOne,
	}
	fb.dlist.SetCombineMode(rdp.CombineMode{
		Two: rdp.CombinePass{RGB: cp, Alpha: cpA},
	})

	fb.dlist.SetOtherModes(
		rdp.ForceBlend|rdp.ImageRead,
		rdp.CycleTypeOne, rdp.RGBDitherNone, rdp.AlphaDitherNone, rdp.ZmodeOpaque, rdp.CvgDestClamp, blendOverPremult,
	)

	fb.dlist.FillRectangle(r)
}

// These modes expect the color combiner to pass (1-alpha) instead of alpha to
// the blender. This allows to calculate all ops with the same color combiner
// configuration.
var (
	blendOver = rdp.BlendMode{ // dst = cc_alpha*dst + (1-cc_alpha)*cc
		P1: rdp.BlenderPMFramebuffer,
		A1: rdp.BlenderAColorCombinerAlpha,
		M1: rdp.BlenderPMColorCombiner,
		B1: rdp.BlenderBOneMinusAlphaA,
	}
	blendOverPremult = rdp.BlendMode{ // dst = cc_alpha*dst + cc
		P1: rdp.BlenderPMFramebuffer,
		A1: rdp.BlenderAColorCombinerAlpha,
		M1: rdp.BlenderPMColorCombiner,
		B1: rdp.BlenderBOne,
	}
	blendSrc = rdp.BlendMode{ // dst = (1-cc_alpha)*cc
		P1: rdp.BlenderPMBlendColor,
		A1: rdp.BlenderAColorCombinerAlpha,
		M1: rdp.BlenderPMColorCombiner,
		B1: rdp.BlenderBOneMinusAlphaA,
	}
	blendSrcPremult = rdp.BlendMode{ // dst = cc
		P1: rdp.BlenderPMFramebuffer,
		A1: rdp.BlenderAZero,
		M1: rdp.BlenderPMColorCombiner,
		B1: rdp.BlenderBOne,
	}
)

func (fb *Rdp) drawColorImage(r image.Rectangle, src *texture.Texture, p image.Point, scale image.Point, fill color.Color, op draw.Op) {
	var modeflags rdp.ModeFlags
	colorSource := rdp.CombineTex0

	if fill != nil {
		fb.dlist.SetPrimitiveColor(fill)
		colorSource = rdp.CombinePrimitive
	}

	if src.Palette() != nil {
		modeflags |= rdp.TLUT
		const tlutIdx = 7
		fb.dlist.SetTextureImage(src.Palette())
		ts := rdp.TileDescriptor{
			Format: texture.CI,
			Size:   texture.BPP4,
			Addr:   0x100,
			Line:   uint16(src.BPP().TMEMWords(src.Palette().Bounds().Dx())),
			Idx:    tlutIdx,
		}
		fb.dlist.SetTile(ts)
		fb.dlist.LoadTLUT(tlutIdx, src.Palette().Bounds())
	}

	var blendmode *rdp.BlendMode
	if op == draw.Over {
		if src.Premult() {
			blendmode = &blendOverPremult
		} else {
			blendmode = &blendOver
		}
		fb.dlist.SetOtherModes(
			rdp.ForceBlend|rdp.ImageRead|rdp.BiLerp0|modeflags,
			rdp.CycleTypeOne, rdp.RGBDitherNone, rdp.AlphaDitherNone, rdp.ZmodeOpaque, rdp.CvgDestClamp, *blendmode,
		)
	} else {
		if src.Premult() {
			blendmode = &blendSrcPremult
		} else {
			blendmode = &blendSrc
			fb.dlist.SetBlendColor(color.RGBA{A: 0xff})
		}
		fb.dlist.SetOtherModes(
			rdp.ForceBlend|rdp.BiLerp0|modeflags,
			rdp.CycleTypeOne, rdp.RGBDitherNone, rdp.AlphaDitherNone, rdp.ZmodeOpaque, rdp.CvgDestClamp, *blendmode,
		)
	}

	fb.dlist.SetCombineMode(rdp.CombineMode{
		Two: rdp.CombinePass{
			RGB: rdp.CombineParams{0, 0, 0, colorSource}, // cc = src
			Alpha: rdp.CombineParams{ // cc_alpha = 1-tex0_alpha
				rdp.CombineAAlphaZero, rdp.CombineBAlphaOne,
				rdp.CombineTex0, rdp.CombineDAlphaOne,
			}},
	})
	fb.dlist.SetTextureImage(src)

	// Loading BPP4 crashes the RDP. As a workaround create two tiles with
	// different BPP, one for loading and one for drawing.
	var loadIdx, drawIdx uint8 = 0, 1
	bpp := max(src.BPP(), texture.BPP8)

	step := rdp.MaxTileSize(src.BPP(), src.Format())
	ts := rdp.TileDescriptor{
		Format: src.Format(),
		Size:   bpp,
		Addr:   0x0,
		Line:   uint16(src.BPP().TMEMWords(step.Dx() / scale.X)),
		Idx:    loadIdx,
	}
	fb.dlist.SetTile(ts)
	if bpp != src.BPP() {
		ts.Size = src.BPP()
		ts.Idx = drawIdx
		fb.dlist.SetTile(ts)
	} else {
		drawIdx = loadIdx
	}

	bounds := src.Bounds().Intersect(r.Sub(r.Min.Sub(p)))
	bounds = bounds.Sub(src.Bounds().Min)        // draw area in src image space
	origin := r.Min.Add(src.Bounds().Min).Sub(p) // draw origin in screen space

	// iterate tile over the whole drawing area
	var pt image.Point
	for pt.X = bounds.Min.X; pt.X < bounds.Max.X; pt.X += step.Dx() {
		for pt.Y = bounds.Min.Y; pt.Y < bounds.Max.Y; pt.Y += step.Dy() {
			tile := step.Add(pt).Intersect(bounds)

			debug.Assert(!tile.Empty(), "drawing empty tile")

			if loadIdx != drawIdx { // load 4bpp texture
				ltile := tile
				ltile.Min.X >>= 1
				ltile.Max.X >>= 1
				fb.dlist.LoadTile(loadIdx, ltile)
				fb.dlist.SetTileSize(drawIdx, tile)
			} else {
				fb.dlist.LoadTile(loadIdx, tile)
			}
			fb.dlist.TextureRectangle(tile.Add(origin), tile.Min, scale, drawIdx)
		}
	}

	// TODO runtime.KeepAlive(src.addr) until FullSync?
}

// Draws text str inside r, beginning at p. Returns the next p.
// Fore- and background colors fg and bg don't support alpha. If a nil
// background color is passed, it will be transparent.
func (fb *Rdp) DrawText(r image.Rectangle, font *fonts.Face, p image.Point, fg, bg color.Color, str []byte) image.Point {
	fb.dlist.SetEnvironmentColor(fg)

	blendmode := &blendOver
	if bg != nil {
		fb.dlist.SetBlendColor(bg)
		blendmode = &blendSrc
	}

	fb.dlist.SetOtherModes(
		rdp.ForceBlend|rdp.ImageRead|rdp.BiLerp0,
		rdp.CycleTypeOne, rdp.RGBDitherNone, rdp.AlphaDitherNone, rdp.ZmodeOpaque, rdp.CvgDestClamp, *blendmode,
	)

	fb.dlist.SetCombineMode(rdp.CombineMode{
		Two: rdp.CombinePass{
			RGB: rdp.CombineParams{0, 0, 0, rdp.CombineEnvironment}, // cc = src
			Alpha: rdp.CombineParams{ // cc_alpha = 1-tex0_alpha
				rdp.CombineAAlphaZero, rdp.CombineBAlphaOne,
				rdp.CombineTex0, rdp.CombineDAlphaOne,
			}},
	})

	const idx = 1
	img, _, _, _ := font.GlyphMap(0)
	if img == nil {
		return p
	}
	tex, ok := img.(*texture.Texture)
	debug.Assert(ok, "fontmap format")
	ts := rdp.TileDescriptor{
		Format: tex.Format(),
		Size:   tex.BPP(),
		Addr:   0x0,
		Line:   uint16(tex.BPP().Bytes(tex.Bounds().Dx()+1) >> 3),
		Idx:    idx,

		MaskS: 5, MaskT: 5, // ignore fractional part
	}
	fb.dlist.SetTile(ts)

	pos := p
	clip := r.Intersect(fb.target.Bounds())

	var oldtex *texture.Texture
	for len(str) > 0 {
		rune, size := utf8.DecodeRune(str)
		str = str[size:]
		if rune == '\n' {
			pos.X = r.Min.X
			pos.Y += int(font.Height)
			continue
		}

		img, glyphRect, _, adv := font.GlyphMap(rune)
		if img == nil {
			continue
		}
		glyphRectSS := image.Rectangle{Max: glyphRect.Size()}.Add(pos)

		if glyphRectSS.Overlaps(clip) {
			tex, ok := img.(*texture.Texture)
			debug.Assert(ok, "fontmap format")
			if tex != oldtex {
				fb.dlist.SetTextureImage(tex)
				oldtex = tex
			}

			fb.dlist.LoadTile(idx, glyphRect)
			fb.dlist.TextureRectangle(glyphRectSS, glyphRect.Min, image.Point{1, 1}, idx)
		}

		pos.X += adv
	}

	return pos
}

func (fb *Rdp) Bounds() image.Rectangle {
	return fb.target.Bounds()
}

func (fb *Rdp) Flush() {
	fb.dlist.Flush()
}
