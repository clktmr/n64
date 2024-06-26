package rdp_test

import (
	"bytes"
	"image"
	"image/draw"
	"image/png"
	"testing"

	"github.com/clktmr/n64/rcp"
	"github.com/clktmr/n64/rcp/rdp"
	"github.com/clktmr/n64/rcp/video"
)

func BenchmarkFillScreen(b *testing.B) {
	rcp.EnableInterrupts(rcp.DisplayProcessor)
	b.Cleanup(func() {
		rcp.DisableInterrupts(rcp.DisplayProcessor)
	})

	fb := video.NewRGBA32(image.Rect(0, 0, video.WIDTH, video.HEIGHT))
	rasterizer := rdp.NewRdp(fb)

	for i := 0; i < b.N; i++ {
		rasterizer.Draw(fb.Rect, image.Black, image.Point{}, nil, image.Point{}, draw.Src)
		rasterizer.Flush()
	}
}

func BenchmarkTextureRectangle(b *testing.B) {
	rcp.EnableInterrupts(rcp.DisplayProcessor)
	b.Cleanup(func() {
		rcp.DisableInterrupts(rcp.DisplayProcessor)
	})

	fb := video.NewRGBA32(image.Rect(0, 0, video.WIDTH, video.HEIGHT))
	rasterizer := rdp.NewRdp(fb)

	imgN64LogoLarge, _ := png.Decode(bytes.NewReader(pngN64LogoLarge))
	imgLarge := video.NewNRGBA32(imgN64LogoLarge.Bounds())
	draw.Src.Draw(&imgLarge.NRGBA, imgLarge.Bounds(), imgN64LogoLarge, image.Point{})
	imgLarge.Flush()

	for i := 0; i < b.N; i++ {
		rasterizer.Draw(fb.Rect, imgLarge, image.Point{}, nil, image.Point{}, draw.Over)
		rasterizer.Flush()
	}
}
