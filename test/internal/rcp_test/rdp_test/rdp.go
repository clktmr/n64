package rdp_test

import (
	_ "embed"
	"image"
	"image/color"
	"testing"

	"github.com/clktmr/n64/rcp/rdp"
	"github.com/clktmr/n64/rcp/texture"
)

func TestFillRect(t *testing.T) {
	testcolor := color.NRGBA{R: 0, G: 0x37, B: 0x77, A: 0xff}
	img := texture.NewFramebuffer(image.Rect(0, 0, 32, 32))

	dl := &rdp.RDP

	dl.SetColorImage(img)

	bounds := image.Rectangle{
		image.Point{0, 0},
		image.Point{10, 5},
	}
	dl.SetScissor(bounds, rdp.InterlaceNone)
	dl.SetFillColor(testcolor)
	dl.SetOtherModes(
		rdp.ForceBlend|rdp.AtomicPrimitive,
		rdp.CycleTypeFill, rdp.RGBDitherNone, rdp.AlphaDitherNone, rdp.ZmodeOpaque, rdp.CvgDestClamp, rdp.BlendMode{},
	)

	img.Invalidate()

	dl.FillRectangle(bounds)
	dl.Flush()

	for x := range bounds.Max.X {
		for y := range bounds.Max.Y {
			result := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			if result != testcolor {
				t.Errorf("%v at (%d,%d)", result, x, y)
			}
		}
	}
}
