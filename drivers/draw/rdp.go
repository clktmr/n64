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

// HW provides hardware accelerated drawing capabilities.
//
// Besides implementing [draw.Drawer], it provides additional functions for
// drawing text efficiently.
type HW draw.Op

const (
	Over = HW(draw.Over)
	Src  = HW(draw.Src)
)

// Draw implements the [Drawer] interface.
func (op HW) Draw(dst draw.Image, r image.Rectangle, src image.Image, sp image.Point) {
	DrawMask(dst, r, src, sp, nil, image.Point{}, draw.Op(op))
}

func (op HW) DrawMask(dst draw.Image, r image.Rectangle, src image.Image, sp image.Point, mask image.Image, mp image.Point) {
	DrawMask(dst, r, src, sp, mask, mp, draw.Op(op))
}

var target *texture.Texture

func Bounds() image.Rectangle { return target.Bounds() }
func Flush()                  { rdp.RDP.Flush() }

func setFramebuffer(tex *texture.Texture) {
	target = tex
	rdp.RDP.SetColorImage(target)
	rdp.RDP.SetScissor(image.Rectangle{Max: target.Bounds().Size()}, rdp.InterlaceNone)
}

func Draw(dst draw.Image, r image.Rectangle, src image.Image, sp image.Point, op draw.Op) {
	DrawMask(dst, r, src, sp, nil, image.Point{}, op)
}

func DrawMask(dst draw.Image, r image.Rectangle, src image.Image, sp image.Point, mask image.Image, mp image.Point, op draw.Op) {
	if target != dst {
		if dst, ok := dst.(*texture.Texture); ok {
			setFramebuffer(dst)
		} else {
			panic("dst not a texture")
		}
	}

	// Readjust r if we draw to a viewport/subimage of the framebuffer
	r = r.Bounds().Sub(dst.Bounds().Min)

	if !r.Overlaps(dst.Bounds()) {
		return
	}

	switch srcImg := src.(type) {
	case *texture.Texture:
		switch mask.(type) {
		case nil:
			_, isAlpha := srcImg.Image.(*image.Alpha)
			if isAlpha {
				drawColorImage(r, srcImg, sp, image.Point{1, 1}, color.RGBA{0xff, 0xff, 0xff, 0xff}, op)
			} else {
				drawColorImage(r, srcImg, sp, image.Point{1, 1}, nil, op)
			}
			return
		}
	case *image.Uniform:
		switch maskImg := mask.(type) {
		case nil:
			// fill
			switch op {
			case draw.Src:
				drawUniformSrc(r, srcImg.C, nil)
				return
			case draw.Over:
				drawUniformOver(r, srcImg.C, color.Opaque)
				return
			}
		case *image.Uniform:
			switch op {
			case draw.Src:
				drawUniformSrc(r, srcImg.C, maskImg.C)
				return
			case draw.Over:
				drawUniformOver(r, srcImg.C, maskImg.C)
				return
			}
		case *texture.Texture:
			drawColorImage(r, maskImg, mp, image.Point{1, 1}, srcImg.C, op)
			return
		case *images.Magnifier:
			maskAlpha, ok := maskImg.Image.(*texture.Texture)
			debug.Assert(ok, "rdp unsupported magnifier format")
			drawColorImage(r, maskAlpha, mp, image.Point{maskImg.Sx, maskImg.Sy}, srcImg.C, op)
			return
		}
	}

	debug.Assert(false, "rdp unsupported format")
}

func drawUniformSrc(r image.Rectangle, fill color.Color, mask color.Color) {
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
	rdp.RDP.SetFillColor(fill)
	rdp.RDP.SetOtherModes(
		0, rdp.CycleTypeFill, rdp.RGBDitherNone, rdp.AlphaDitherNone, rdp.ZmodeOpaque, rdp.CvgDestClamp, rdp.BlendMode{},
	)
	rdp.RDP.FillRectangle(r)
}

func drawUniformOver(r image.Rectangle, fill color.Color, mask color.Color) {
	// CycleTypeFill doesn't support blending, use CycleTypeOne instead. The
	// following operation is required by draw.Over:
	//
	//     a = 1.0 - (fill_alpha * mask_alpha)
	//     dst = (dst*a + fill*mask_alpha)

	rdp.RDP.SetPrimitiveColor(fill)
	rdp.RDP.SetEnvironmentColor(mask)

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
	rdp.RDP.SetCombineMode(rdp.CombineMode{
		Two: rdp.CombinePass{RGB: cp, Alpha: cpA},
	})

	rdp.RDP.SetOtherModes(
		rdp.ForceBlend|rdp.ImageRead,
		rdp.CycleTypeOne, rdp.RGBDitherNone, rdp.AlphaDitherNone, rdp.ZmodeOpaque, rdp.CvgDestClamp, blendOverPremult,
	)

	rdp.RDP.FillRectangle(r)
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

func drawColorImage(r image.Rectangle, src *texture.Texture, p image.Point, scale image.Point, fill color.Color, op draw.Op) {
	var modeflags rdp.ModeFlags
	colorSource := rdp.CombineTex0

	if fill != nil {
		rdp.RDP.SetPrimitiveColor(fill)
		colorSource = rdp.CombinePrimitive
	}

	if src.Palette() != nil {
		modeflags |= rdp.TLUT
		rdp.RDP.SetTextureImage(src.Palette())
		_, idx := rdp.RDP.SetTile(rdp.TileDescriptor{
			Format: texture.CI4, // tlut must always use 4bpp
			Addr:   0x100,
			Line:   uint16(src.Format().TMEMWords(src.Palette().Bounds().Dx())),
		})
		rdp.RDP.LoadTLUT(idx, src.Palette().Bounds())
	}

	var blendmode *rdp.BlendMode
	if op == draw.Over {
		if src.Premult() {
			blendmode = &blendOverPremult
		} else {
			blendmode = &blendOver
		}
		rdp.RDP.SetOtherModes(
			rdp.ForceBlend|rdp.ImageRead|rdp.BiLerp0|modeflags,
			rdp.CycleTypeOne, rdp.RGBDitherNone, rdp.AlphaDitherNone, rdp.ZmodeOpaque, rdp.CvgDestClamp, *blendmode,
		)
	} else {
		if src.Premult() {
			blendmode = &blendSrcPremult
		} else {
			blendmode = &blendSrc
			rdp.RDP.SetBlendColor(color.RGBA{A: 0xff})
		}
		rdp.RDP.SetOtherModes(
			rdp.ForceBlend|rdp.BiLerp0|modeflags,
			rdp.CycleTypeOne, rdp.RGBDitherNone, rdp.AlphaDitherNone, rdp.ZmodeOpaque, rdp.CvgDestClamp, *blendmode,
		)
	}

	rdp.RDP.SetCombineMode(rdp.CombineMode{
		Two: rdp.CombinePass{
			RGB: rdp.CombineParams{0, 0, 0, colorSource}, // cc = src
			Alpha: rdp.CombineParams{ // cc_alpha = 1-tex0_alpha
				rdp.CombineAAlphaZero, rdp.CombineBAlphaOne,
				rdp.CombineTex0, rdp.CombineDAlphaOne,
			}},
	})
	rdp.RDP.SetTextureImage(src)

	step := rdp.MaxTileSize(src.Format())
	loadIdx, drawIdx := rdp.RDP.SetTile(rdp.TileDescriptor{
		Format: src.Format(),
		Addr:   0x0,
		Line:   uint16(src.Format().TMEMWords(step.Dx() / scale.X)),
	})

	bounds := src.Bounds().Intersect(r.Sub(r.Min.Sub(p)))
	bounds = bounds.Sub(src.Bounds().Min)        // draw area in src image space
	origin := r.Min.Add(src.Bounds().Min).Sub(p) // draw origin in screen space

	// iterate tile over the whole drawing area
	var pt image.Point
	for pt.X = bounds.Min.X; pt.X < bounds.Max.X; pt.X += step.Dx() {
		for pt.Y = bounds.Min.Y; pt.Y < bounds.Max.Y; pt.Y += step.Dy() {
			tile := step.Add(pt).Intersect(bounds)

			debug.Assert(!tile.Empty(), "drawing empty tile")

			rdp.RDP.LoadTile(loadIdx, tile)
			rdp.RDP.TextureRectangle(tile.Add(origin), tile.Min, scale, drawIdx)
		}
	}

	// TODO runtime.KeepAlive(src.addr) until FullSync?
}

// Draws text str inside r, beginning at p. Returns the next p.
// Fore- and background colors fg and bg don't support alpha. If a nil
// background color is passed, it will be transparent.
func DrawText(dst image.Image, r image.Rectangle, font *fonts.Face, p image.Point, fg, bg color.Color, str []byte) image.Point {
	if target != dst {
		if dst, ok := dst.(*texture.Texture); ok {
			setFramebuffer(dst)
		} else {
			panic("dst not a texture")
		}
	}

	rdp.RDP.SetEnvironmentColor(fg)

	blendmode := &blendOver
	if bg != nil {
		rdp.RDP.SetBlendColor(bg)
		blendmode = &blendSrc
	}

	rdp.RDP.SetOtherModes(
		rdp.ForceBlend|rdp.ImageRead|rdp.BiLerp0,
		rdp.CycleTypeOne, rdp.RGBDitherNone, rdp.AlphaDitherNone, rdp.ZmodeOpaque, rdp.CvgDestClamp, *blendmode,
	)

	rdp.RDP.SetCombineMode(rdp.CombineMode{
		Two: rdp.CombinePass{
			RGB: rdp.CombineParams{0, 0, 0, rdp.CombineEnvironment}, // cc = src
			Alpha: rdp.CombineParams{ // cc_alpha = 1-tex0_alpha
				rdp.CombineAAlphaZero, rdp.CombineBAlphaOne,
				rdp.CombineTex0, rdp.CombineDAlphaOne,
			}},
	})

	img, _, _, _ := font.GlyphMap(0)
	if img == nil {
		return p
	}
	tex, ok := img.(*texture.Texture)
	debug.Assert(ok, "fontmap format")
	loadIdx, drawIdx := rdp.RDP.SetTile(rdp.TileDescriptor{
		Format: tex.Format(),
		Addr:   0x0,
		Line:   uint16(tex.Format().TMEMWords(tex.Bounds().Dx())),
	})

	pos := p
	clip := r.Intersect(dst.Bounds())

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
				rdp.RDP.SetTextureImage(tex)
				oldtex = tex
			}

			rdp.RDP.LoadTile(loadIdx, glyphRect)
			rdp.RDP.TextureRectangle(glyphRectSS, glyphRect.Min, image.Point{1, 1}, drawIdx)
		}

		pos.X += adv
	}

	return pos
}
