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

// Op is a Porter-Duff compositing operator which facilitates RDP acceleration
// for images of type [*github.com/clktmr/n64/rcp/texture.Texture].
type Op draw.Op

const (
	Over = Op(draw.Over)
	Src  = Op(draw.Src)
)

// Draw implements the [draw.Drawer] interface.
func (op Op) Draw(dst draw.Image, r image.Rectangle, src image.Image, sp image.Point) {
	DrawMask(dst, r, src, sp, nil, image.Point{}, draw.Op(op))
}

func (op Op) DrawMask(dst draw.Image, r image.Rectangle, src image.Image, sp image.Point, mask image.Image, mp image.Point) {
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
	fb, ok := dst.(*texture.Texture)
	if !ok {
		goto fallback
	}

	setFramebuffer(fb)

	if !r.Overlaps(fb.Bounds()) {
		return
	}

	// Move r into framebuffer's coordinate space
	r = r.Sub(fb.Bounds().Min)

	switch srcImg := src.(type) {
	case *texture.Texture:
		switch mask.(type) {
		case nil:
			_, isAlpha := srcImg.Image.(*image.Alpha)
			if isAlpha {
				drawColorImage(r, srcImg, sp, image.Point{1, 1}, color.NRGBA{0xff, 0xff, 0xff, 0xff}, op)
			} else {
				drawColorImage(r, srcImg, sp, image.Point{1, 1}, color.NRGBA{}, op)
			}
			return
		}
	case *TextImage:
		srcImg.Draw(fb, r, sp, op)
		return
	case *image.Uniform:
		switch maskImg := mask.(type) {
		case nil:
			// fill
			switch op {
			case draw.Src:
				c := color.RGBAModel.Convert(srcImg.C).(color.RGBA)
				drawUniformSrc(r, c, color.RGBA{A: 0xff})
				return
			case draw.Over:
				c := color.NRGBAModel.Convert(srcImg.C).(color.NRGBA)
				drawUniformOver(r, c, color.NRGBA{A: 0xff})
				return
			}
		case *image.Uniform:
			switch op {
			case draw.Src:
				c := color.RGBAModel.Convert(srcImg.C).(color.RGBA)
				m := color.RGBAModel.Convert(maskImg.C).(color.RGBA)
				drawUniformSrc(r, c, m)
				return
			case draw.Over:
				c := color.NRGBAModel.Convert(srcImg.C).(color.NRGBA)
				m := color.NRGBAModel.Convert(maskImg.C).(color.NRGBA)
				drawUniformOver(r, c, m)
				return
			}
		case *texture.Texture:
			c := color.NRGBAModel.Convert(srcImg.C).(color.NRGBA)
			drawColorImage(r, maskImg, mp, image.Point{1, 1}, c, op)
			return
		case *images.Magnifier:
			maskAlpha, ok := maskImg.Image.(*texture.Texture)
			debug.Assert(ok, "rdp unsupported magnifier format")
			c := color.NRGBAModel.Convert(srcImg.C).(color.NRGBA)
			drawColorImage(r, maskAlpha, mp, image.Point{maskImg.Sx, maskImg.Sy}, c, op)
			return
		}
	}

fallback:
	if tex, ok := src.(*texture.Texture); ok {
		src = tex.Image
	}
	if tex, ok := mask.(*texture.Texture); ok {
		mask = tex.Image
	}
	draw.DrawMask(dst, r, src, sp, mask, mp, draw.Op(op))
}

func drawUniformSrc(r image.Rectangle, fill, mask color.RGBA) {
	if mask.A != 0xff {
		ma := uint16(mask.A) + 1
		fill.R = uint8((uint16(fill.R) * ma) >> 8)
		fill.G = uint8((uint16(fill.G) * ma) >> 8)
		fill.B = uint8((uint16(fill.B) * ma) >> 8)
		fill.A = uint8((uint16(fill.A) * ma) >> 8)
	}
	rdp.RDP.SetFillColor(fill)
	rdp.RDP.SetOtherModes(rdp.OtherModes(
		0, rdp.CycleTypeFill, rdp.RGBDitherNone, rdp.AlphaDitherNone, rdp.ZmodeOpaque, rdp.CvgDestClamp, rdp.BlendMode(0),
	))
	rdp.RDP.SetScissor(r, rdp.InterlaceNone)
	rdp.RDP.FillRectangle(r)
}

func drawUniformOver(r image.Rectangle, fill, mask color.NRGBA) {
	// CycleTypeFill doesn't support blending, use CycleTypeOne instead.
	// Assuming fill is not premultiplied alpha, the following operation is
	// required by draw.Over:
	//
	//     a = (fill_alpha * mask_alpha)
	//     dst = (1-a)*dst + a*fill

	rdp.RDP.SetPrimitiveColor(fill)
	rdp.RDP.SetEnvironmentColor(mask)

	rdp.RDP.SetCombineMode(rdp.CombineMode1Cycle(
		0, 0, 0, rdp.CombinePrimitive,
		rdp.CombinePrimitive, rdp.CombineBAlphaZero, rdp.CombineEnvironment, rdp.CombineDAlphaZero,
	))

	rdp.RDP.SetOtherModes(rdp.OtherModes(
		rdp.ForceBlend|rdp.ImageRead,
		rdp.CycleTypeOne, rdp.RGBDitherNone, rdp.AlphaDitherNone, rdp.ZmodeOpaque, rdp.CvgDestClamp, blendOver(),
	))

	rdp.RDP.SetScissor(r, rdp.InterlaceNone)
	rdp.RDP.FillRectangle(r)
}

// dst = cc_alpha*cc + (1-cc_alpha)*dst
func blendOver() rdp.BlendMode {
	return rdp.BlendMode1Cycle(
		rdp.BlenderPMColorCombiner,
		rdp.BlenderPMFramebuffer,
		rdp.BlenderAColorCombinerAlpha,
		rdp.BlenderBOneMinusAlphaA,
	)
}

// dst = cc_alpha*cc
func blendSrc() rdp.BlendMode {
	return rdp.BlendMode1Cycle(
		rdp.BlenderPMColorCombiner,
		rdp.BlenderPMColorCombiner,
		rdp.BlenderAColorCombinerAlpha,
		rdp.BlenderBZero,
	)
}

// dst = cc_alpha*cc + (1-cc_alpha)*env
func blendOverEnv() rdp.BlendMode {
	return rdp.BlendMode1Cycle(
		rdp.BlenderPMColorCombiner,
		rdp.BlenderPMBlendColor,
		rdp.BlenderAColorCombinerAlpha,
		rdp.BlenderBOneMinusAlphaA,
	)
}

func drawColorImage(r image.Rectangle, src *texture.Texture, p image.Point, scale image.Point, fill color.NRGBA, op draw.Op) {
	var modeflags rdp.ModeFlags
	colorSource := rdp.CombineTex0
	alphaSource := rdp.CombineTex0

	if fill.A != 0x0 {
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
		rdp.RDP.SetOtherModes(rdp.OtherModes(
			rdp.ForceBlend|rdp.ImageRead|rdp.BiLerp0|modeflags,
			rdp.CycleTypeOne, rdp.RGBDitherNone, rdp.AlphaDitherPattern, rdp.ZmodeOpaque, rdp.CvgDestClamp, blendOver(),
		))
	} else {
		rdp.RDP.SetOtherModes(rdp.OtherModes(
			rdp.ForceBlend|rdp.BiLerp0|modeflags,
			rdp.CycleTypeOne, rdp.RGBDitherNone, rdp.AlphaDitherNone, rdp.ZmodeOpaque, rdp.CvgDestClamp, blendSrc(),
		))
	}

	if !src.HasAlpha() {
		alphaSource = rdp.CombineDAlphaOne
	}
	rdp.RDP.SetCombineMode(rdp.CombineMode1Cycle(
		0, 0, 0, colorSource,
		0, 0, 0, alphaSource,
	))
	rdp.RDP.SetTextureImage(src)

	step := rdp.MaxTileSize(src.Format())
	loadIdx, drawIdx := rdp.RDP.SetTile(rdp.TileDescriptor{
		Format: src.Format(),
		Addr:   0x0,
		Line:   uint16(src.Format().TMEMWords(step.X / scale.X)),
	})

	trans := r.Min.Sub(p)
	scissor := r.Intersect(src.Bounds().Add(trans))
	rdp.RDP.SetScissor(scissor, rdp.InterlaceNone)

	trans = trans.Add(src.Bounds().Min) // texture image space to screen space
	bounds := scissor.Sub(trans)        // draw area in texture image space

	// iterate tile over the whole drawing area
	var pt image.Point
	for pt.X = bounds.Min.X; pt.X < bounds.Max.X; pt.X += step.X {
		for pt.Y = bounds.Min.Y; pt.Y < bounds.Max.Y; pt.Y += step.Y {
			tile := image.Rectangle{pt, pt.Add(step)}

			debug.Assert(!tile.Empty(), "drawing empty tile")

			rdp.RDP.LoadTile(loadIdx, tile)
			rdp.RDP.TextureRectangle(tile.Add(trans), tile.Min, scale, drawIdx)
		}
	}
}

// Draws text str inside r, beginning at p. Returns the next p.
// Fore- and background colors fg and bg don't support alpha. If a nil
// background color is passed, it will be transparent.
func DrawText(dst image.Image, r image.Rectangle, font *fonts.Face, p image.Point, fg, bg color.NRGBA, str []byte) image.Point {
	if dst, _ := dst.(*texture.Texture); true {
		setFramebuffer(dst)
	}

	rdp.RDP.SetPrimitiveColor(fg)

	var blendmode rdp.BlendMode
	mode := rdp.ForceBlend | rdp.BiLerp0
	if bg.A == 0x0 {
		mode |= rdp.ImageRead
		blendmode = blendOver()
	} else {
		rdp.RDP.SetBlendColor(bg)
		blendmode = blendOverEnv()
	}
	rdp.RDP.SetOtherModes(rdp.OtherModes(mode,
		rdp.CycleTypeOne, rdp.RGBDitherNone, rdp.AlphaDitherNone, rdp.ZmodeOpaque, rdp.CvgDestClamp, blendmode,
	))

	rdp.RDP.SetCombineMode(rdp.CombineMode1Cycle(
		0, 0, 0, rdp.CombinePrimitive,
		0, 0, 0, rdp.CombineTex0,
	))

	tex, _ := font.GlyphMap(0)
	if tex == nil {
		return p
	}
	loadIdx, drawIdx := rdp.RDP.SetTile(rdp.TileDescriptor{
		Format: tex.Format(),
		Addr:   0x0,
		Line:   uint16(tex.Format().TMEMWords(tex.Bounds().Dx())),
	})

	pos := p
	scissor := r.Intersect(dst.Bounds())
	rdp.RDP.SetScissor(scissor, rdp.InterlaceNone)

	outofbounds := false
	var oldtex *texture.Texture
	// process characters in batches for better cache efficiency
	var batch [8]struct {
		rune  rune
		tex   *texture.Texture
		glyph *fonts.Glyph
	}
	for len(str) > 0 {
		i := 0
		ppos := pos
		for i = range batch {
			v := &batch[i]
			size := 0
			v.rune, size = utf8.DecodeRune(str)
			str = str[size:]
			if v.rune == '\n' {
				ppos.X = r.Min.X
				ppos.Y += int(font.Height)
				outofbounds = false
				continue
			}
			if outofbounds {
				v.tex = nil
				continue
			}

			v.tex, v.glyph = font.GlyphMap(v.rune)
			ppos.X += int(v.glyph.Advance)
			if ppos.X > scissor.Max.X {
				outofbounds = true // skip rest of line
			}
			if len(str) == 0 {
				break
			}
		}

		for i := range i + 1 {
			v := &batch[i]
			if v.rune == '\n' {
				pos.X = r.Min.X
				pos.Y += int(font.Height)
				continue
			}
			if v.tex == nil {
				continue
			}

			var drawRect image.Rectangle
			var sp image.Point
			if bg.A != 0x0 {
				glyphRect := image.Rect(0, int(font.Height-font.Ascent), int(v.glyph.Advance), int(-font.Ascent))
				drawRect = glyphRect.Add(pos)
				sp = glyphRect.Min.Add(v.glyph.Origin.Pt())
			} else {
				drawRect = v.glyph.Rect.Rect().Add(pos.Sub(v.glyph.Origin.Pt()))
				sp = v.glyph.Rect.Min.Pt()
			}

			if v.tex != oldtex {
				rdp.RDP.SetTextureImage(v.tex)
				oldtex = v.tex
			}
			rdp.RDP.LoadTile(loadIdx, v.glyph.Rect.Rect())
			rdp.RDP.TextureRectangle(drawRect, sp, image.Point{1, 1}, drawIdx)

			pos.X += int(v.glyph.Advance)
		}
	}

	return pos
}
