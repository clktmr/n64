package draw_test

import (
	"bytes"
	_ "embed"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"testing"

	n64draw "github.com/clktmr/n64/drivers/draw"
	"github.com/clktmr/n64/rcp"
	"github.com/clktmr/n64/rcp/texture"
	"github.com/clktmr/n64/rcp/video"
)

//go:embed testdata/gradient.png
var pngN64LogoSmall []byte

//go:embed testdata/n64.png
var pngN64LogoLarge []byte

// Fills an image with a checkerboard test pattern
func checkerboard(img *texture.RGBA32) {
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
			draw.Src.Draw(&img.RGBA, square, &image.Uniform{colors[i]}, image.Point{})
			square = square.Add(image.Point{0, size})
		}
		squareStart = squareStart.Add(image.Point{size, 0})
	}
	img.Writeback()
}

func absDiffInt(a int, b int) int {
	diff := a - b
	return max(diff, -diff)
}

// Returns the absolute difference in RGB.  Alpha channel is ignored.
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
	fb := texture.NewRGBA32(image.Rect(0, 0, video.WIDTH, video.HEIGHT))
	bounds := image.Rect(0, 0, video.WIDTH/2, video.HEIGHT/2)
	expected := fb.SubImage(bounds)
	result := fb.SubImage(bounds.Add(image.Point{video.WIDTH / 2, 0}))
	result.Rect = bounds
	diff := fb.SubImage(bounds.Add(image.Point{0, video.HEIGHT / 2}))
	diff.Rect = bounds
	err := fb.SubImage(bounds.Add(image.Point{video.WIDTH / 2, video.HEIGHT / 2}))
	err.Rect = bounds

	video.SetupPAL(fb)

	// Load some test images
	imgN64LogoSmall, _ := png.Decode(bytes.NewReader(pngN64LogoSmall))
	imgN64LogoLarge, _ := png.Decode(bytes.NewReader(pngN64LogoLarge))

	imgGreen := &image.Uniform{color.RGBA{G: 0xff, A: 0xff}}
	imgTransparentGreen := &image.Uniform{color.RGBA{G: 0xff, A: 0xaf}}
	imgTransparentGray := &image.Uniform{color.RGBA{0x7f, 0x7f, 0x7f, 0xaf}}

	imgNRGBA := texture.NewNRGBA32(imgN64LogoSmall.Bounds())
	draw.Src.Draw(&imgNRGBA.NRGBA, imgNRGBA.Bounds(), imgN64LogoSmall, image.Point{})
	imgNRGBA.Writeback()
	imgRGBA := texture.NewRGBA32(imgN64LogoSmall.Bounds())
	draw.Src.Draw(&imgRGBA.RGBA, imgRGBA.Bounds(), imgN64LogoSmall, image.Point{})
	imgRGBA.Writeback()

	imgLarge := texture.NewNRGBA32(imgN64LogoLarge.Bounds())
	draw.Src.Draw(&imgLarge.NRGBA, imgLarge.Bounds(), imgN64LogoLarge, image.Point{})
	imgLarge.Writeback()

	imgAlpha := texture.NewI8(imgN64LogoSmall.Bounds())
	draw.Src.Draw(imgAlpha, imgAlpha.Bounds(), imgN64LogoSmall, image.Point{})
	imgAlpha.Writeback()

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
	drawerHW := n64draw.NewRdp(result)
	drawerSW := n64draw.NewCpu(expected)
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// prepare
			draw.Src.Draw(&diff.RGBA, bounds, imgTransparentGray, image.Point{})
			draw.Src.Draw(&err.RGBA, bounds, image.Black, image.Point{})
			checkerboard(expected)
			checkerboard(result)
			result.Invalidate()

			// draw
			drawerSW.Draw(tc.r, tc.src, tc.sp, tc.mask, tc.mp, tc.op) // expected
			drawerSW.Flush()
			drawerHW.Draw(tc.r, tc.src, tc.sp, tc.mask, tc.mp, tc.op) // result
			drawerHW.Flush()

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
