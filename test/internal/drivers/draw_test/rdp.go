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
	"github.com/clktmr/n64/rcp/texture"
	"github.com/clktmr/n64/rcp/video"
)

//go:embed testdata/gradient.png
var pngN64LogoSmall []byte

//go:embed testdata/n64.png
var pngN64LogoLarge []byte

// Fills an image with a checkerboard test pattern
func checkerboard(img *texture.Texture) {
	const size = 16
	b := img.Bounds()
	colors := []color.RGBA{
		{0x7f, 0x7f, 0x0, 0x0},
		{0x0, 0x0, 0x7f, 0x7f},
	}
	squareStart := image.Rect(0, 0, size, size).Add(img.Bounds().Min)
	for x := b.Min.X; x < b.Max.X; x += size {
		square := squareStart
		for y := b.Min.Y; y < b.Max.Y; y += size {
			i := (x/size + y/size) % 2
			draw.Src.Draw(img.Image, square, &image.Uniform{colors[i]}, image.Point{})
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

// Returns the absolute difference in RGB. Alpha channel is ignored.
func absDiffColor(a color.Color, b color.Color) int {
	ac := color.RGBAModel.Convert(a).(color.RGBA)
	bc := color.RGBAModel.Convert(b).(color.RGBA)

	return absDiffInt(int(ac.R), int(bc.R)) +
		absDiffInt(int(ac.G), int(bc.G)) +
		absDiffInt(int(ac.B), int(bc.B))
}

func TestDrawMask(t *testing.T) {
	// Split the screen into four viewports
	fb := texture.NewRGBA32(image.Rect(0, 0, 320, 240))
	quarter := image.Rectangle{Max: fb.Bounds().Max.Div(2)}
	bounds := quarter.Inset(16)
	expected := fb.SubImage(bounds)
	expected.SetOrigin(bounds.Min)
	result := fb.SubImage(bounds.Add(image.Pt(quarter.Max.X, 0)))
	result.SetOrigin(bounds.Min)
	diff := fb.SubImage(bounds.Add(image.Pt(0, quarter.Max.Y)))
	diff.SetOrigin(bounds.Min)
	err := fb.SubImage(bounds.Add(quarter.Max))
	err.SetOrigin(bounds.Min)

	video.SetupPAL(false, false)
	video.SetFramebuffer(fb)
	t.Cleanup(func() { video.SetFramebuffer(nil) })

	// Load some test images
	imgN64LogoSmall, _ := png.Decode(bytes.NewReader(pngN64LogoSmall))
	imgN64LogoLarge, _ := png.Decode(bytes.NewReader(pngN64LogoLarge))

	imgGreen := &image.Uniform{color.RGBA{G: 0xff, A: 0xff}}
	imgTransparentGreen := &image.Uniform{color.RGBA{G: 0xff, A: 0xaf}}
	imgTransparentGray := &image.Uniform{color.RGBA{0x7f, 0x7f, 0x7f, 0xaf}}

	imgNRGBA := texture.NewNRGBA32(imgN64LogoSmall.Bounds())
	draw.Src.Draw(imgNRGBA.Image, imgNRGBA.Bounds(), imgN64LogoSmall, image.Point{})
	imgNRGBA.Writeback()
	imgRGBA := texture.NewRGBA32(imgN64LogoSmall.Bounds())
	draw.Src.Draw(imgRGBA.Image, imgRGBA.Bounds(), imgN64LogoSmall, image.Point{})
	imgRGBA.Writeback()

	imgLarge := texture.NewNRGBA32(imgN64LogoLarge.Bounds())
	draw.Src.Draw(imgLarge.Image, imgLarge.Bounds(), imgN64LogoLarge, image.Point{})
	imgLarge.Writeback()

	imgLarge16 := texture.NewRGBA16(imgN64LogoLarge.Bounds())
	draw.Src.Draw(imgLarge16.Image, imgLarge16.Bounds(), imgN64LogoLarge, image.Point{})
	imgLarge16.Writeback()

	imgLarge8 := texture.NewI8(imgN64LogoLarge.Bounds())
	draw.Src.Draw(imgLarge8.Image, imgLarge8.Bounds(), imgN64LogoLarge, image.Point{})
	imgLarge8.Writeback()

	imgLarge4 := texture.NewI4(imgN64LogoLarge.Bounds())
	draw.Src.Draw(imgLarge4.Image, imgLarge4.Bounds(), imgN64LogoLarge, image.Point{})
	imgLarge4.Writeback()

	imgAlpha := texture.NewAlpha(imgN64LogoSmall.Bounds())
	draw.Src.Draw(imgAlpha, imgAlpha.Bounds(), imgN64LogoSmall, image.Point{})
	imgAlpha.Writeback()

	imgI4 := texture.NewI4(imgN64LogoSmall.Bounds())
	draw.Src.Draw(imgI4, imgI4.Bounds(), imgN64LogoSmall, image.Point{})
	imgI4.Writeback()

	// Define testcases
	tests := map[string]struct {
		r    image.Rectangle
		src  image.Image
		sp   image.Point
		mask image.Image
		mp   image.Point
		op   draw.Op
	}{
		"fillSrc":              {bounds.Inset(24), imgTransparentGreen, image.Point{}, nil, image.Point{}, draw.Src},
		"fillOver":             {bounds.Inset(24), imgTransparentGreen, image.Point{}, nil, image.Point{}, draw.Over},
		"fillMaskSrc":          {bounds.Inset(24), imgTransparentGreen, image.Point{}, imgTransparentGray, image.Point{}, draw.Src},
		"fillMaskOver":         {bounds.Inset(24), imgTransparentGreen, image.Point{}, imgTransparentGray, image.Point{}, draw.Over},
		"fillAlphaMaskSrc":     {bounds.Inset(24), imgGreen, image.Point{}, imgAlpha, image.Point{}, draw.Src},
		"fillAlphaMaskOver":    {bounds.Inset(24), imgGreen, image.Point{}, imgAlpha, image.Point{}, draw.Over},
		"fillOutOfBounds":      {bounds.Inset(-4), imgGreen, image.Point{11, 5}, nil, image.Point{}, draw.Src},
		"drawSrc":              {bounds.Inset(24), imgNRGBA, image.Point{}, nil, image.Point{}, draw.Src},
		"drawOver":             {bounds.Inset(24), imgNRGBA, image.Point{}, nil, image.Point{}, draw.Over},
		"drawSrcPremult":       {bounds.Inset(24), imgRGBA, image.Point{}, nil, image.Point{}, draw.Src},
		"drawOverPremult":      {bounds.Inset(24), imgRGBA, image.Point{}, nil, image.Point{}, draw.Over},
		"drawSrcSubimage":      {bounds.Inset(24), imgNRGBA.SubImage(imgNRGBA.Bounds().Inset(4)), image.Point{}, nil, image.Point{}, draw.Src},
		"drawSrcSubimageShift": {bounds.Inset(24), imgNRGBA.SubImage(imgNRGBA.Bounds().Inset(4)), image.Point{11, 5}, nil, image.Point{}, draw.Src},
		"drawScissored":        {imgNRGBA.Bounds().Add(image.Pt(24, 24)).Inset(2), imgNRGBA, image.Point{}, nil, image.Point{}, draw.Src},
		"drawLarge":            {bounds.Inset(24), imgLarge, image.Point{}, nil, image.Point{}, draw.Src},
		"drawLarge16":          {bounds.Inset(24), imgLarge16, image.Point{}, nil, image.Point{}, draw.Src},
		"drawLarge8":           {bounds.Inset(24), imgLarge8, image.Point{}, nil, image.Point{}, draw.Src},
		"drawLarge4":           {bounds.Inset(0), imgLarge4, image.Point{4, 4}, nil, image.Point{}, draw.Src},
		"drawShift":            {bounds.Inset(24), imgNRGBA, image.Point{11, 5}, nil, image.Point{}, draw.Src},
		"drawOutOfBoundsUL":    {bounds.Inset(-4), imgNRGBA, image.Point{11, 5}, nil, image.Point{}, draw.Src},
		"drawOutOfBoundsLR":    {bounds.Add(bounds.Size().Sub(image.Point{12, 12})), imgNRGBA, image.Point{11, 5}, nil, image.Point{}, draw.Src},
		"drawSrcI4":            {bounds.Inset(24), imgI4, image.Point{}, nil, image.Point{}, draw.Src},
		"drawSrcAlpha":         {bounds.Inset(24), imgAlpha, image.Point{}, nil, image.Point{}, draw.Src},
		"drawOverAlpha":        {bounds.Inset(24), imgAlpha, image.Point{}, nil, image.Point{}, draw.Over},
	}

	// Run all testcases
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// prepare
			draw.Src.Draw(diff.Image, bounds, imgTransparentGray, image.Point{})
			draw.Src.Draw(err.Image, bounds, image.Black, image.Point{})
			checkerboard(expected)
			checkerboard(result)
			result.Invalidate()

			// draw
			n64draw.SW(tc.op).DrawMask(expected, tc.r, tc.src, tc.sp, tc.mask, tc.mp) // expected
			n64draw.HW(tc.op).DrawMask(result, tc.r, tc.src, tc.sp, tc.mask, tc.mp)   // result
			n64draw.Flush()

			// compare
			const showThreshold = 18 // allow some errors due to precision
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
						})
					}
				}
			}
			if cumErr > 0 {
				t.Errorf("images do not match, see video output for details")
				t.Fatalf("cumulative error: %v", cumErr)
			}
		})

	}
}
