package draw_test

import (
	"image"
	"image/color"
	"testing"

	"github.com/clktmr/n64/drivers/draw"
	"github.com/clktmr/n64/fonts/gomono12"
	"github.com/clktmr/n64/rcp/texture"
	"github.com/clktmr/n64/rcp/video"
)

var lorem = []byte(`Lorem ipsum dolor sit amet, consectetur adipisici elit, sed
eiusmod tempor incidunt ut labore et dolore magna aliqua. Ut enim ad
minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquid
ex ea commodi consequat. Quis aute iure reprehenderit in voluptate velit
esse cillum dolore eu fugiat nulla pariatur. Excepteur sint obcaecat
cupiditat non proident, sunt in culpa qui officia deserunt mollit anim
id est laborum.`)

func BenchmarkDrawText(b *testing.B) {
	gomono := gomono12.NewFace()

	fb := texture.NewRGBA16(image.Rect(0, 0, 320, 240))

	video.Setup(false)
	video.SetFramebuffer(fb)
	b.Cleanup(func() { video.SetFramebuffer(nil) })
	white := color.RGBA{0xff, 0xff, 0xff, 0xff}
	black := color.RGBA{0x0, 0x0, 0x0, 0xff}

	for b.Loop() {
		draw.DrawText(fb, fb.Bounds(), gomono, image.Point{}, white, black, lorem)
	}
}
