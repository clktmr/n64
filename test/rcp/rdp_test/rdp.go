package rdp_test

import (
	"bytes"
	_ "embed"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"testing"
	"unsafe"

	"github.com/clktmr/n64/framebuffer"
	"github.com/clktmr/n64/rcp"
	"github.com/clktmr/n64/rcp/cpu"
	"github.com/clktmr/n64/rcp/rdp"
	"github.com/clktmr/n64/rcp/video"
)

//go:embed testdata/gradient.png
var pngN64LogoSmall []byte

//go:embed testdata/n64.png
var pngN64LogoLarge []byte

func TestFillRect(t *testing.T) {
	rcp.EnableInterrupts(rcp.DisplayProcessor)
	t.Cleanup(func() {
		rcp.DisableInterrupts(rcp.DisplayProcessor)
	})

	testcolor := color.RGBA{R: 0, G: 0x37, B: 0x77, A: 0xff}
	img := framebuffer.NewRGBA32(image.Rect(0, 0, 32, 32))
	imgBuf := uintptr(unsafe.Pointer(&img.Pix[:1][0]))

	dl := rdp.NewDisplayList()

	dl.SetColorImage(imgBuf, 32, rdp.RGBA, rdp.BBP32)

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

// Fills an image with a checkerboard test pattern
func checkerboard(img video.Drawer) {
	const size = 16
	b := img.Bounds()
	colors := []color.RGBA{
		{0x7f, 0x7f, 0x0, 0x0},
		{0x0, 0x0, 0x7f, 0x7f},
	}
	squareStart := image.Rect(0, 0, size, size)
	for x := b.Min.X; x < b.Max.X; x += size {
		square := squareStart
		for y := b.Min.Y; y < b.Max.Y; y += size {
			i := (x/size + y/size) % 2
			img.Draw(square, &image.Uniform{colors[i]}, image.Point{}, nil, image.Point{}, draw.Src)
			square = square.Add(image.Point{0, size})
		}
		squareStart = squareStart.Add(image.Point{size, 0})
	}
	img.Flush()
}

func absDiffInt(a int, b int) int {
	diff := a - b
	if diff < 0 {
		return -diff
	}
	return diff
}

// Returns true if both colors are almost identical.  Alpha channel is ignored.
func absDiffColor(a color.Color, b color.Color) int {
	ac := color.RGBAModel.Convert(a).(color.RGBA)
	bc := color.RGBAModel.Convert(b).(color.RGBA)

	return absDiffInt(int(ac.R), int(bc.R)) +
		absDiffInt(int(ac.G), int(bc.G)) +
		absDiffInt(int(ac.B), int(bc.B))
}

func TestDraw(t *testing.T) {
	rcp.EnableInterrupts(rcp.DisplayProcessor)
	t.Cleanup(func() {
		rcp.DisableInterrupts(rcp.DisplayProcessor)
	})

	// Split the screen into four viewports
	fb := video.NewRGBA32(image.Rect(0, 0, video.WIDTH, video.HEIGHT))
	bounds := image.Rect(0, 0, video.WIDTH/2, video.HEIGHT/2)
	expected := fb.SubImage(bounds).(*video.RGBA32)
	result := fb.SubImage(bounds.Add(image.Point{video.WIDTH / 2, 0})).(*video.RGBA32)
	result.Rect = bounds
	diff := fb.SubImage(bounds.Add(image.Point{0, video.HEIGHT / 2})).(*video.RGBA32)
	diff.Rect = bounds
	err := fb.SubImage(bounds.Add(image.Point{video.WIDTH / 2, video.HEIGHT / 2})).(*video.RGBA32)
	err.Rect = bounds

	video.SetFramebuffer(fb)
	video.SetupPAL(video.BBP32)

	// Load some test images
	imgN64LogoSmall, _ := png.Decode(bytes.NewReader(pngN64LogoSmall))
	imgN64LogoLarge, _ := png.Decode(bytes.NewReader(pngN64LogoLarge))

	imgGreen := &image.Uniform{color.RGBA{G: 0xff, A: 0xff}}
	imgTransparentGreen := &image.Uniform{color.RGBA{G: 0xff, A: 0xaf}}
	imgTransparentGray := &image.Uniform{color.RGBA{0x7f, 0x7f, 0x7f, 0xaf}}
	imgNRGBA := video.NewNRGBA32(imgN64LogoSmall.Bounds())
	draw.Src.Draw(&imgNRGBA.NRGBA, imgNRGBA.Bounds(), imgN64LogoSmall, image.Point{})
	imgNRGBA.Flush()
	imgRGBA := video.NewRGBA32(imgN64LogoSmall.Bounds())
	draw.Src.Draw(&imgRGBA.RGBA, imgRGBA.Bounds(), imgN64LogoSmall, image.Point{})
	imgRGBA.Flush()

	imgLarge := video.NewNRGBA32(imgN64LogoLarge.Bounds())
	draw.Src.Draw(&imgLarge.NRGBA, imgLarge.Bounds(), imgN64LogoLarge, image.Point{})
	imgLarge.Flush()

	imgAlpha := image.NewAlpha(imgN64LogoSmall.Bounds())
	draw.Src.Draw(imgAlpha, imgAlpha.Bounds(), imgN64LogoSmall, image.Point{})
	// FIXME imgAlpha.Flush()

	// Define testcases
	tests := map[string]struct {
		r    image.Rectangle
		src  image.Image
		sp   image.Point
		mask image.Image
		mp   image.Point
		op   draw.Op

		threshold int // Allow some errors due to precision
	}{
		"fillSrc":              {bounds.Inset(24), imgTransparentGreen, image.Point{}, nil, image.Point{}, draw.Src, 0},
		"fillOver":             {bounds.Inset(24), imgTransparentGreen, image.Point{}, nil, image.Point{}, draw.Over, 0},
		"fillMaskSrc":          {bounds.Inset(24), imgTransparentGreen, image.Point{}, imgTransparentGray, image.Point{}, draw.Src, 0},
		"fillMaskOver":         {bounds.Inset(24), imgTransparentGreen, image.Point{}, imgTransparentGray, image.Point{}, draw.Over, 0},
		"fillAlphaMaskSrc":     {bounds.Inset(24), imgGreen, image.Point{}, imgAlpha, image.Point{}, draw.Src, 0},
		"fillAlphaMaskOver":    {bounds.Inset(24), imgGreen, image.Point{}, imgAlpha, image.Point{}, draw.Over, 1800},
		"drawSrc":              {bounds.Inset(24), imgNRGBA, image.Point{}, nil, image.Point{}, draw.Src, 0},
		"drawOver":             {bounds.Inset(24), imgNRGBA, image.Point{}, nil, image.Point{}, draw.Over, 2600},
		"drawSrcPremult":       {bounds.Inset(24), imgRGBA, image.Point{}, nil, image.Point{}, draw.Src, 0},
		"drawOverPremult":      {bounds.Inset(24), imgRGBA, image.Point{}, nil, image.Point{}, draw.Over, 1800},
		"drawSrcSubimage":      {bounds.Inset(24), imgNRGBA.SubImage(imgNRGBA.Rect.Inset(4)), image.Point{}, nil, image.Point{}, draw.Src, 0},
		"drawSrcSubimageShift": {bounds.Inset(24), imgNRGBA.SubImage(imgNRGBA.Rect.Inset(4)), image.Point{11, 5}, nil, image.Point{}, draw.Src, 0},
		"drawScissored":        {imgNRGBA.Rect.Add(image.Pt(24, 24)).Inset(2), imgNRGBA, image.Point{}, nil, image.Point{}, draw.Src, 0},
		"drawLarge":            {bounds.Inset(24), imgLarge, image.Point{}, nil, image.Point{}, draw.Src, 100},
		"drawShift":            {bounds.Inset(24), imgNRGBA, image.Point{11, 5}, nil, image.Point{}, draw.Src, 0},
		// TODO "drawSrcI8":            {bounds.Inset(24), imgAlpha, image.Point{}, nil, image.Point{}, draw.Over, 0},
	}

	// Run all testcases
	rasterizer := rdp.NewRdp(result)
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// prepare
			diff.Draw(bounds, imgTransparentGray, image.Point{}, nil, image.Point{}, draw.Src)
			err.Draw(bounds, image.Black, image.Point{}, nil, image.Point{}, draw.Src)
			checkerboard(expected)
			checkerboard(result)

			// draw
			expected.Draw(tc.r, tc.src, tc.sp, tc.mask, tc.mp, tc.op) // expected
			expected.Flush()
			cpu.InvalidateSlice(result.Pix)
			rasterizer.Draw(tc.r, tc.src, tc.sp, tc.mask, tc.mp, tc.op) // result
			rasterizer.Flush()

			// compare
			const showThreshold = 3
			cumErr := 0
			for x := range bounds.Max.X {
				for y := range bounds.Max.Y {
					e := expected.At(x, y).(color.RGBA)
					r := result.At(x, y).(color.RGBA)
					absErr := absDiffColor(e, r)
					if absErr > showThreshold {
						cumErr += absErr
						err.Set(x, y, color.RGBA{R: 0xff})
						diff.Set(x, y, color.RGBA{
							R: 0x7f + e.R/2 - r.R/2,
							G: 0x7f + e.G/2 - r.G/2,
							B: 0x7f + e.B/2 - r.B/2,
							// A: 0x7f + e.A/2 - r.A/2,
						})
					}
				}
			}
			if cumErr > tc.threshold {
				t.Errorf("images do not match, see video output for details")
				t.Fatalf("cumulative error: %v", cumErr)
			}
		})

	}
}
