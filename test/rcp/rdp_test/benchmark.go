package rdp_test

import (
	"image"
	"image/draw"
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
