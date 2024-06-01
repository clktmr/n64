package rdp_test

import (
	"image"
	"image/color"
	"n64/rcp"
	"n64/rcp/cpu"
	"n64/rcp/rdp"
	"testing"
	"unsafe"
)

func TestFillRect(t *testing.T) {
	rcp.EnableInterrupts(rcp.DisplayProcessor)
	t.Cleanup(func() {
		rcp.DisableInterrupts(rcp.DisplayProcessor)
	})

	testcolor := color.RGBA{R: 0, G: 0x37, B: 0x77, A: 0xff}
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	imgBuf := uintptr(unsafe.Pointer(&img.Pix[:1][0]))

	rdp.SetColorImage(imgBuf, 32, rdp.RGBA, rdp.BBP32)

	bounds := image.Rectangle{
		image.Point{0, 0},
		image.Point{10, 5},
	}
	rdp.SetScissor(bounds, rdp.InterlaceNone)
	rdp.SetFillColor(testcolor)
	rdp.SetOtherModes(rdp.RGBDitherNone |
		rdp.AlphaDitherNone | rdp.ForceBlend |
		rdp.CycleTypeFill | rdp.AtomicPrimitive)
	rdp.FillRectangle(bounds)

	rdp.Run()

	cpu.Invalidate(imgBuf, img.Stride*32)

	for x := range bounds.Max.X {
		for y := range bounds.Max.Y {
			result := img.At(x, y)
			if result != testcolor {
				t.Errorf("%v at (%d,%d)", result, x, y)
			}
		}
	}
}
