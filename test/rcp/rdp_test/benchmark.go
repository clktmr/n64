package rdp_test

import (
	"bytes"
	"image"
	"image/draw"
	"image/png"
	"testing"

	"github.com/clktmr/n64/rcp"
	"github.com/clktmr/n64/rcp/cpu"
	"github.com/clktmr/n64/rcp/rdp"
	"github.com/clktmr/n64/rcp/texture"
	"github.com/clktmr/n64/rcp/video"
)

func BenchmarkFillScreen(b *testing.B) {
	rcp.EnableInterrupts(rcp.DisplayProcessor)
	b.Cleanup(func() {
		rcp.DisableInterrupts(rcp.DisplayProcessor)
	})

	fb := texture.NewRGBA32(image.Rect(0, 0, video.WIDTH, video.HEIGHT))
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

	fb := texture.NewRGBA32(image.Rect(0, 0, video.WIDTH, video.HEIGHT))
	rasterizer := rdp.NewRdp(fb)

	imgN64LogoLarge, _ := png.Decode(bytes.NewReader(pngN64LogoLarge))
	imgLarge := texture.NewNRGBA32(imgN64LogoLarge.Bounds())
	draw.Src.Draw(&imgLarge.NRGBA, imgLarge.Bounds(), imgN64LogoLarge, image.Point{})
	cpu.WritebackSlice(imgLarge.Pix)

	for i := 0; i < b.N; i++ {
		rasterizer.Draw(fb.Rect, imgLarge, image.Point{}, nil, image.Point{}, draw.Over)
		rasterizer.Flush()
	}
}
