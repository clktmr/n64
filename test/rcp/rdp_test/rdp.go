package rdp_test

import (
	_ "embed"
	"image"
	"image/color"
	"testing"

	"github.com/clktmr/n64/rcp"
	"github.com/clktmr/n64/rcp/cpu"
	"github.com/clktmr/n64/rcp/rdp"
	"github.com/clktmr/n64/rcp/texture"
)

func TestFillRect(t *testing.T) {
	rcp.EnableInterrupts(rcp.DisplayProcessor)
	t.Cleanup(func() {
		rcp.DisableInterrupts(rcp.DisplayProcessor)
	})

	testcolor := color.RGBA{R: 0, G: 0x37, B: 0x77, A: 0xff}
	img := texture.NewRGBA32(image.Rect(0, 0, 32, 32))

	dl := rdp.NewDisplayList()

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
	dl.FillRectangle(bounds)

	cpu.InvalidateSlice(img.Pix)

	rdp.Run(dl)

	for x := range bounds.Max.X {
		for y := range bounds.Max.Y {
			result := img.At(x, y)
			if result != testcolor {
				t.Errorf("%v at (%d,%d)", result, x, y)
			}
		}
	}
}
