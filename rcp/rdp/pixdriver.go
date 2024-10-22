package rdp

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"

	"github.com/clktmr/n64/debug"
	"github.com/clktmr/n64/rcp/texture"

	"github.com/embeddedgo/display/images"
)

type Rdp struct {
	target texture.Texture
	bounds image.Rectangle
	dlist  *DisplayList

	fill image.Uniform
}

func NewRdp(fb texture.Texture) *Rdp {
	rdp := &Rdp{
		target: fb,
		dlist:  NewDisplayList(),
	}

	rdp.dlist.SetColorImage(fb)
	rdp.dlist.SetScissor(rdp.target.Bounds().Sub(rdp.target.Bounds().Min), InterlaceNone)

	return rdp
}

func (fb *Rdp) Draw(r image.Rectangle, src image.Image, sp image.Point, mask image.Image, mp image.Point, op draw.Op) {
	if len(fb.dlist.commands) >= DisplayListLength/2 { // TODO ugly
		fb.Flush()
	}

	// Readjust r if we draw to a viewport/subimage of the framebuffer
	r = r.Bounds().Sub(fb.target.Bounds().Min)

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
			debug.Assert(ok, fmt.Sprintf("rdp unsupported format: magnified %T", maskAlpha))
			fb.drawColorImage(r, maskAlpha, mp, image.Point{maskImg.Sx, maskImg.Sy}, srcImg.C, op)
			return
		}
	}

	debug.Assert(false, fmt.Sprintf("rdp unsupported format: %T with %T mask", src, mask))
}

func (fb *Rdp) drawUniformSrc(r image.Rectangle, fill color.Color, mask color.Color) {
	if mask != nil {
		rf, gf, bf, af := fill.RGBA()
		_, _, _, ma := mask.RGBA()
		m := uint32(ma)
		fill = color.RGBA64{
			uint16((rf * m) >> 16),
			uint16((gf * m) >> 16),
			uint16((bf * m) >> 16),
			uint16((af * m) >> 16),
		}
	}
	fb.dlist.SetFillColor(fill)
	fb.dlist.SetOtherModes(
		0, CycleTypeFill, RGBDitherNone, AlphaDitherNone, ZmodeOpaque, CvgDestClamp, BlendMode{},
	)
	fb.dlist.FillRectangle(r)
}

func (fb *Rdp) drawUniformOver(r image.Rectangle, fill color.Color, mask color.Color) {
	// CycleTypeFill doesn't support blending, use CycleTypeOne instead. The
	// following operation is required by draw.Over:
	//
	//     a = 1.0 - (fill_alpha * mask_alpha)
	//     dst = (dst*a + fill*mask_alpha)
	//
	// We will calculate `a` with the ColorCombiner alpha pass, which
	// calculates (A-B)*C+D.  The following code sets A=0.0, B=mask_alpha,
	// C=fill_alpha and D=1.0.
	//
	// We can also calculate fill*mask_alpha with the ColorCombiner rgb
	// pass by setting A=fill, B=0.0, C=mask_alpha, D=0.0.
	//
	// The remaining `dst` operation can be easily configured to be
	// calculated by the Blender.

	fb.dlist.SetPrimitiveColor(fill)
	fb.dlist.SetEnvironmentColor(mask)

	cp := CombineParams{CombinePrimitive, CombineBColorZero, CombineCColorEnvironmentAlpha, CombineDColorZero} // cc = env_alpha*primitive_color
	cpA := CombineParams{CombineAAlphaZero, CombineEnvironment, CombinePrimitive, CombineDAlphaOne}            // cc_alpha = 1-env_alpha*primitive_alpha
	fb.dlist.SetCombineMode(CombineMode{
		Two: CombinePass{RGB: cp, Alpha: cpA},
	})

	fb.dlist.SetOtherModes(
		ForceBlend|ImageRead,
		CycleTypeOne, RGBDitherNone, AlphaDitherNone, ZmodeOpaque, CvgDestClamp,
		BlendMode{ // dst = cc_alpha*dst + cc
			P1: BlenderPMFramebuffer,
			A1: BlenderAColorCombinerAlpha,
			M1: BlenderPMColorCombiner,
			B1: BlenderBOne,
		},
	)

	fb.dlist.FillRectangle(r)
}

func (fb *Rdp) drawColorImage(r image.Rectangle, src texture.Texture, p image.Point, scale image.Point, fill color.Color, op draw.Op) {
	colorSource := CombineTex0

	if fill != nil {
		fb.dlist.SetEnvironmentColor(fill)
		colorSource = CombineEnvironment
	}

	var cp CombineParams
	if src.Premult() {
		cp = CombineParams{0, 0, 0, colorSource} // cc = src
	} else {
		cp = CombineParams{colorSource, CombineBColorZero, CombineCColorTex0Alpha, CombineDColorZero} // cc = src_alpha*src
	}

	if op == draw.Over {
		fb.dlist.SetOtherModes(
			ForceBlend|ImageRead|BiLerp0,
			CycleTypeOne, RGBDitherNone, AlphaDitherNone, ZmodeOpaque, CvgDestClamp,
			BlendMode{ // dst = cc_alpha*dst + cc
				P1: BlenderPMFramebuffer,
				A1: BlenderAColorCombinerAlpha,
				M1: BlenderPMColorCombiner,
				B1: BlenderBOne,
			},
		)
		cpA := CombineParams{CombineAAlphaZero, CombineBAlphaOne, CombineTex0, CombineDAlphaOne} // cc_alpha = 1-tex0_alpha

		fb.dlist.SetCombineMode(CombineMode{
			Two: CombinePass{RGB: cp, Alpha: cpA},
		})
	} else {
		fb.dlist.SetOtherModes(
			ForceBlend|BiLerp0,
			CycleTypeOne, RGBDitherNone, AlphaDitherNone, ZmodeOpaque, CvgDestClamp,
			BlendMode{ // dst = cc
				A1: BlenderAZero,
				M1: BlenderPMColorCombiner,
				B1: BlenderBOne,
			},
		)
		cpA := CombineParams{0, 0, 0, colorSource} // cc_alpha = src_alpha
		fb.dlist.SetCombineMode(CombineMode{
			Two: CombinePass{RGB: cp, Alpha: cpA},
		})
	}

	fb.dlist.SetTextureImage(src)

	step := maxTile(src.BPP())
	const idx = 0
	ts := TileDescriptor{
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

func (fb *Rdp) Flush() {
	Run(fb.dlist)
	fb.dlist.commands = fb.dlist.commands[:2] // TODO ugly displaylist reset
}

func maxTile(bpp texture.BitDepth) image.Rectangle {
	size := 256 >> uint(bpp>>51)
	return image.Rect(0, 0, size, size)
}

func (fb *Rdp) Fill(rect image.Rectangle) {
	fb.Draw(rect, &fb.fill, image.Point{}, nil, image.Point{}, draw.Over)
}

func (fb *Rdp) SetColor(c color.Color) {
	fb.fill.C = c
}

func (fb *Rdp) SetDir(dir int) image.Rectangle {
	return fb.target.Bounds()
}

func (fb *Rdp) Err(clear bool) error {
	return nil
}
