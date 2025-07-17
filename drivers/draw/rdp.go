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

// Draw implements the [draw.Drawer] interface.
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
	if dst, _ := dst.(*texture.Texture); true {
		setFramebuffer(dst)
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
	// CycleTypeFill doesn't support blending, use CycleTypeOne instead.
	// Assuming fill is not premultiplied alpha, the following operation is
	// required by draw.Over:
	//
	//     a = (fill_alpha * mask_alpha)
	//     dst = (1-a)*dst + a*fill

	rdp.RDP.SetPrimitiveColor(fill)
	rdp.RDP.SetEnvironmentColor(mask)

	rdp.RDP.SetCombineMode(rdp.CombineMode{
		Two: rdp.CombinePass{
			RGB:   rdp.CombineParams{0, 0, 0, rdp.CombinePrimitive},
			Alpha: rdp.CombineParams{rdp.CombinePrimitive, rdp.CombineBAlphaZero, rdp.CombineEnvironment, rdp.CombineDAlphaZero},
		},
	})

	rdp.RDP.SetOtherModes(
		rdp.ForceBlend|rdp.ImageRead,
		rdp.CycleTypeOne, rdp.RGBDitherNone, rdp.AlphaDitherNone, rdp.ZmodeOpaque, rdp.CvgDestClamp, blendOver,
	)

	rdp.RDP.FillRectangle(r)
}

var (
	blendOver = rdp.BlendMode{ // dst = cc_alpha*cc + (1-cc_alpha)*dst
		P1: rdp.BlenderPMColorCombiner,
		A1: rdp.BlenderAColorCombinerAlpha,
		M1: rdp.BlenderPMFramebuffer,
		B1: rdp.BlenderBOneMinusAlphaA,
	}
	blendSrc = rdp.BlendMode{ // dst = cc_alpha*cc
		P1: rdp.BlenderPMColorCombiner,
		A1: rdp.BlenderAColorCombinerAlpha,
		M1: rdp.BlenderPMColorCombiner,
		B1: rdp.BlenderBZero,
	}
	blendOverEnv = rdp.BlendMode{ // dst = cc_alpha*cc + (1-cc_alpha)*env
		P1: rdp.BlenderPMColorCombiner,
		A1: rdp.BlenderAColorCombinerAlpha,
		M1: rdp.BlenderPMBlendColor,
		B1: rdp.BlenderBOneMinusAlphaA,
	}
)

func drawColorImage(r image.Rectangle, src *texture.Texture, p image.Point, scale image.Point, fill color.Color, op draw.Op) {
	var modeflags rdp.ModeFlags
	colorSource := rdp.CombineTex0
	alphaSource := rdp.CombineTex0

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

	if op == draw.Over {
		rdp.RDP.SetOtherModes(
			rdp.ForceBlend|rdp.ImageRead|rdp.BiLerp0|modeflags,
			rdp.CycleTypeOne, rdp.RGBDitherNone, rdp.AlphaDitherPattern, rdp.ZmodeOpaque, rdp.CvgDestClamp, blendOver,
		)
	} else {
		rdp.RDP.SetOtherModes(
			rdp.ForceBlend|rdp.BiLerp0|modeflags,
			rdp.CycleTypeOne, rdp.RGBDitherNone, rdp.AlphaDitherNone, rdp.ZmodeOpaque, rdp.CvgDestClamp, blendSrc,
		)
	}

	if !src.HasAlpha() {
		alphaSource = rdp.CombineDAlphaOne
	}
	rdp.RDP.SetCombineMode(rdp.CombineMode{
		Two: rdp.CombinePass{
			RGB:   rdp.CombineParams{0, 0, 0, colorSource},
			Alpha: rdp.CombineParams{0, 0, 0, alphaSource},
		},
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
}

// Draws text str inside r, beginning at p. Returns the next p.
// Fore- and background colors fg and bg don't support alpha. If a nil
// background color is passed, it will be transparent.
func DrawText(dst image.Image, r image.Rectangle, font *fonts.Face, p image.Point, fg, bg color.Color, str []byte) image.Point {
	if dst, _ := dst.(*texture.Texture); true {
		setFramebuffer(dst)
	}

	rdp.RDP.SetPrimitiveColor(fg)

	blendmode := &blendOver
	if bg != nil {
		rdp.RDP.SetBlendColor(bg)
		blendmode = &blendOverEnv
	}

	rdp.RDP.SetOtherModes(
		rdp.ForceBlend|rdp.ImageRead|rdp.BiLerp0,
		rdp.CycleTypeOne, rdp.RGBDitherNone, rdp.AlphaDitherNone, rdp.ZmodeOpaque, rdp.CvgDestClamp, *blendmode,
	)

	rdp.RDP.SetCombineMode(rdp.CombineMode{
		Two: rdp.CombinePass{
			RGB:   rdp.CombineParams{0, 0, 0, rdp.CombinePrimitive},
			Alpha: rdp.CombineParams{0, 0, 0, rdp.CombineTex0},
		},
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

	outofbounds := false
	var oldtex *texture.Texture
	for len(str) > 0 {
		rune, size := utf8.DecodeRune(str)
		str = str[size:]
		if rune == '\n' {
			pos.X = r.Min.X
			pos.Y += int(font.Height)
			outofbounds = false
			continue
		}
		if outofbounds {
			continue
		}

		img, glyphRect, origin, adv := font.GlyphMap(rune)
		if img == nil {
			continue
		}

		glyphRectSS := glyphRect.Sub(origin).Add(pos)
		var drawRect image.Rectangle
		if bg != nil {
			cellRect := image.Rect(0, int(font.Height-font.Ascent), adv, int(-font.Ascent))
			drawRect = cellRect.Add(pos).Intersect(clip)
		} else {
			drawRect = glyphRectSS.Intersect(clip)
		}
		if !drawRect.Empty() {
			tex, ok := img.(*texture.Texture)
			debug.Assert(ok, "fontmap format")
			if tex != oldtex {
				rdp.RDP.SetTextureImage(tex)
				oldtex = tex
			}

			sp := glyphRect.Min.Add(drawRect.Min.Sub(glyphRectSS.Min))

			rdp.RDP.LoadTile(loadIdx, glyphRect)
			rdp.RDP.TextureRectangle(drawRect, sp, image.Point{1, 1}, drawIdx)
		} else if glyphRectSS.Min.X > clip.Max.X {
			outofbounds = true // skip rest of line
		}

		pos.X += adv
	}

	return pos
}
