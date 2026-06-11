package draw

import (
	"image"
	"image/color"
	"image/draw"

	"github.com/clktmr/n64/rcp/rdp"
)

// BorderImage is an image type that draws a border of a configurable color and width.
// The width of each of the four borders (Top, Bottom, Left, Right) can be configured separately.
type BorderImage struct {
	Width, Height int
	Color         color.NRGBA
	Inner         image.Rectangle
}

var _ image.Image = &BorderImage{}

// NewBorderImage returns a new BorderImage of the given size, color and border widths.
func NewBorderImage(w, h int, c color.Color, top, bottom, left, right int) *BorderImage {
	w, h = max(w, left+right), max(h, top+bottom)
	return &BorderImage{
		Width:  w,
		Height: h,
		Color:  color.NRGBAModel.Convert(c).(color.NRGBA),
		Inner:  image.Rect(left, top, w-right, h-bottom),
	}
}

func (b *BorderImage) ColorModel() color.Model {
	return color.NRGBAModel
}

func (b *BorderImage) Bounds() image.Rectangle {
	return image.Rect(0, 0, b.Width, b.Height)
}

func (b *BorderImage) At(x, y int) color.Color {
	pt := image.Pt(x, y)
	if pt.In(b.Inner) || !pt.In(image.Rect(0, 0, b.Width, b.Height)) {
		return color.NRGBA{}
	}
	return b.Color
}

func (b *BorderImage) SetSize(w, h int) {
	w, h = max(w, b.Inner.Dx()), max(h, b.Inner.Dy())
	br := b.Inner.Max
	bottom, right := b.Height-br.Y, b.Width-br.X
	b.Width, b.Height = w, h
	b.Inner.Max = image.Pt(w-right, h-bottom)
}

func drawBorderImage(r image.Rectangle, src *BorderImage, sp image.Point, op draw.Op) {
	rects := [4]image.Rectangle{
		image.Rect(0, 0, src.Width, src.Inner.Min.Y),                             // Top
		image.Rect(0, src.Inner.Max.Y, src.Width, src.Height),                    // Bottom
		image.Rect(0, src.Inner.Min.Y, src.Inner.Min.X, src.Inner.Max.Y),         // Left
		image.Rect(src.Inner.Max.X, src.Inner.Min.Y, src.Width, src.Inner.Max.Y), // Right
	}

	rdp.RDP.SetScissor(r, rdp.InterlaceNone)

	pos := r.Min.Sub(sp)
	switch op {
	case draw.Src:
		rdp.RDP.SetOtherModes(rdp.OtherModes(
			0, rdp.CycleTypeFill, rdp.RGBDitherNone, rdp.AlphaDitherNone, rdp.ZmodeOpaque, rdp.CvgDestClamp, blendSrc(),
		))

		rdp.RDP.SetFillColor(color.RGBA{})
		rdp.RDP.FillRectangle(src.Inner.Add(pos))

		fill := color.RGBAModel.Convert(src.Color).(color.RGBA)
		rdp.RDP.SetFillColor(fill)
	case draw.Over:
		rdp.RDP.SetPrimitiveColor(src.Color)
		if src.Color.A == 0xff {
			rdp.RDP.SetOtherModes(rdp.OtherModes(
				0, rdp.CycleTypeFill, rdp.RGBDitherNone, rdp.AlphaDitherNone, rdp.ZmodeOpaque, rdp.CvgDestClamp, blendSrc(),
			))
		} else {
			rdp.RDP.SetCombineMode(rdp.CombineMode1Cycle(
				0, 0, 0, rdp.CombinePrimitive,
				rdp.CombinePrimitive, rdp.CombineBAlphaZero, rdp.CombineEnvironment, rdp.CombineDAlphaZero,
			))
			rdp.RDP.SetOtherModes(rdp.OtherModes(
				rdp.ForceBlend|rdp.ImageRead,
				rdp.CycleTypeOne, rdp.RGBDitherNone, rdp.AlphaDitherNone, rdp.ZmodeOpaque, rdp.CvgDestClamp, blendOver(),
			))
		}
	}

	for _, drawRect := range rects {
		rdp.RDP.FillRectangle(drawRect.Add(pos))
	}
}
