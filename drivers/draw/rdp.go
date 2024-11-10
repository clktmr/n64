package draw

import (
	"image"
	"image/color"
	"image/draw"

	"github.com/clktmr/n64/debug"
	"github.com/clktmr/n64/fonts"
	"github.com/clktmr/n64/rcp/rdp"
	"github.com/clktmr/n64/rcp/texture"

	"github.com/embeddedgo/display/images"
)

type Rdp struct {
	target texture.Texture
	dlist  *rdp.DisplayList
}

func NewRdp() *Rdp {
	r := &Rdp{
		dlist: &rdp.RDP,
	}

	return r
}

func (fb *Rdp) SetFramebuffer(tex texture.Texture) {
	fb.target = tex
	fb.dlist.SetColorImage(fb.target)
	fb.dlist.SetScissor(image.Rectangle{Max: fb.target.Bounds().Size()}, rdp.InterlaceNone)
}

func (fb *Rdp) Draw(r image.Rectangle, src image.Image, sp image.Point, mask image.Image, mp image.Point, op draw.Op) {
	// Readjust r if we draw to a viewport/subimage of the framebuffer
	r = r.Bounds().Sub(fb.target.Bounds().Min)

	if !r.Overlaps(fb.target.Bounds()) {
		return
	}

	switch srcImg := src.(type) {
	case texture.Texture:
		switch mask.(type) {
		case nil:
			fb.drawColorImage(r, srcImg, sp, image.Point{1, 1}, nil, op)
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
		case *texture.I8:
			fb.drawColorImage(r, maskImg, mp, image.Point{1, 1}, srcImg.C, op)
			return
		case *images.Magnifier:
			maskAlpha, ok := maskImg.Image.(*texture.I8)
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
		rdp.CycleTypeOne, rdp.RGBDitherNone, rdp.AlphaDitherNone, rdp.ZmodeOpaque, rdp.CvgDestClamp, blendOver,
	)

	fb.dlist.FillRectangle(r)
}

// These modes expect the color combiner to pass to the blender:
//
//	RGB: src image as premultiplied alpha
//	Alpha: 1-src_alpha
var (
	blendOver = rdp.BlendMode{ // dst = cc_alpha*dst + cc
		P1: rdp.BlenderPMFramebuffer,
		A1: rdp.BlenderAColorCombinerAlpha,
		M1: rdp.BlenderPMColorCombiner,
		B1: rdp.BlenderBOne,
	}
	blendSrc = rdp.BlendMode{ // dst = cc
		A1: rdp.BlenderAZero,
		M1: rdp.BlenderPMColorCombiner,
		B1: rdp.BlenderBOne,
	}
)

func (fb *Rdp) drawColorImage(r image.Rectangle, src texture.Texture, p image.Point, scale image.Point, fill color.Color, op draw.Op) {
	colorSource := rdp.CombineTex0

	if fill != nil {
		fb.dlist.SetEnvironmentColor(fill)
		colorSource = rdp.CombineEnvironment
	}

	var cp rdp.CombineParams
	if src.Premult() {
		cp = rdp.CombineParams{0, 0, 0, colorSource} // cc = src
	} else {
		cp = rdp.CombineParams{colorSource, rdp.CombineBColorZero, rdp.CombineCColorTex0Alpha, rdp.CombineDColorZero} // cc = src_alpha*src
	}

	if op == draw.Over {
		fb.dlist.SetOtherModes(
			rdp.ForceBlend|rdp.ImageRead|rdp.BiLerp0,
			rdp.CycleTypeOne, rdp.RGBDitherNone, rdp.AlphaDitherNone, rdp.ZmodeOpaque, rdp.CvgDestClamp, blendOver,
		)
	} else {
		fb.dlist.SetOtherModes(
			rdp.ForceBlend|rdp.BiLerp0,
			rdp.CycleTypeOne, rdp.RGBDitherNone, rdp.AlphaDitherNone, rdp.ZmodeOpaque, rdp.CvgDestClamp, blendSrc,
		)
	}

	fb.dlist.SetCombineMode(rdp.CombineMode{
		Two: rdp.CombinePass{RGB: cp, Alpha: rdp.CombineParams{ // cc_alpha = 1-tex0_alpha
			rdp.CombineAAlphaZero, rdp.CombineBAlphaOne,
			rdp.CombineTex0, rdp.CombineDAlphaOne,
		}},
	})
	fb.dlist.SetTextureImage(src)

	step := rdp.MaxTileSize(src.BPP())
	const idx = 0
	ts := rdp.TileDescriptor{
		Format: src.Format(),
		Size:   src.BPP(),
		Addr:   0x0,
		Line:   uint16(texture.PixelsToBytes(step.Dx()/scale.X, src.BPP()) >> 3),
		Idx:    idx,

		MaskS: 5, MaskT: 5, // ignore fractional part
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

			fb.dlist.SetTile(ts)
			fb.dlist.LoadTile(idx, tile)
			fb.dlist.TextureRectangle(tile.Add(origin), tile.Min, scale, idx)
		}
	}

	// TODO runtime.KeepAlive(src.addr) until FullSync?
}

func (fb *Rdp) DrawText(r image.Rectangle, font *fonts.Face, p image.Point, c color.Color, str string) image.Point {
	fb.dlist.SetEnvironmentColor(c)
	colorSource := rdp.CombineEnvironment

	// cc = src_alpha*src
	cp := rdp.CombineParams{
		colorSource, rdp.CombineBColorZero,
		rdp.CombineCColorTex0Alpha, rdp.CombineDColorZero,
	}
	// cc_alpha = 1-tex0_alpha
	cpA := rdp.CombineParams{
		rdp.CombineAAlphaZero, rdp.CombineBAlphaOne,
		rdp.CombineTex0, rdp.CombineDAlphaOne,
	}

	fb.dlist.SetOtherModes(
		rdp.ForceBlend|rdp.ImageRead|rdp.BiLerp0,
		rdp.CycleTypeOne, rdp.RGBDitherNone, rdp.AlphaDitherNone, rdp.ZmodeOpaque, rdp.CvgDestClamp, blendOver,
	)

	fb.dlist.SetCombineMode(rdp.CombineMode{
		Two: rdp.CombinePass{RGB: cp, Alpha: cpA},
	})

	const idx = 1
	img, _, _, _ := font.GlyphMap(0)
	tex, ok := img.(texture.Texture)
	debug.Assert(ok, "fontmap format")
	ts := rdp.TileDescriptor{
		Format: tex.Format(),
		Size:   tex.BPP(),
		Addr:   0x0,
		Line:   uint16(texture.PixelsToBytes(tex.Bounds().Dx()+1, tex.BPP()) >> 3),
		Idx:    idx,

		MaskS: 5, MaskT: 5, // ignore fractional part
	}
	fb.dlist.SetTile(ts)

	pos := p
	clip := r.Intersect(fb.target.Bounds())

	var oldtex texture.Texture
	for _, rune := range str {
		if rune == '\n' {
			pos.X = r.Min.X
			pos.Y += int(font.Height)
			continue
		}

		img, glyphRect, _, adv := font.GlyphMap(rune)
		glyphRectSS := image.Rectangle{Max: glyphRect.Size()}.Add(pos)

		if glyphRectSS.Overlaps(clip) {
			tex, ok := img.(texture.Texture)
			debug.Assert(ok, "fontmap format")
			if tex != oldtex {
				fb.dlist.SetTextureImage(tex)
				oldtex = tex
			}

			fb.dlist.LoadTile(idx, glyphRect)
			fb.dlist.TextureRectangle(glyphRectSS, glyphRect.Min, image.Point{1, 1}, idx)
		}

		pos.X += adv
		if pos.X > r.Max.X {
			pos.X = r.Min.X
			pos.Y += int(font.Height)
			if pos.Y > clip.Max.Y {
				break
			}
		}
	}

	return pos
}

func (fb *Rdp) Bounds() image.Rectangle {
	return fb.target.Bounds()
}

func (fb *Rdp) Flush() {
	fb.dlist.Flush()
}
