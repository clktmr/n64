package console

import (
	"bytes"
	"image"

	"github.com/clktmr/n64/drivers/controller"
	"github.com/clktmr/n64/drivers/draw"
	"github.com/clktmr/n64/fonts/basicfont12"
	"github.com/clktmr/n64/rcp/rdp"
	"github.com/clktmr/n64/rcp/serial/joybus"
	"github.com/clktmr/n64/rcp/video"
)

type Console struct {
	buf    bytes.Buffer
	scroll image.Point
}

var font = basicfont12.NewFace()

func NewConsole() *Console { return &Console{} }

func (v *Console) Write(p []byte) (n int, err error) {
	n, err = v.buf.Write(p)
	v.Draw()
	rdp.RDP.Flush()
	return
}

func (v *Console) Update(input controller.Controller) {
	pressed := input.Pressed()
	switch {
	case pressed&joybus.ButtonCUp != 0:
		v.scroll.Y += 1
	case pressed&joybus.ButtonCDown != 0:
		v.scroll.Y -= 1
	case pressed&joybus.ButtonCLeft != 0:
		v.scroll.X += int(font.Advance(0))
	case pressed&joybus.ButtonCRight != 0:
		v.scroll.X -= int(font.Advance(0))
	}
}

// FIXME sync via mutex with Write?
func (v *Console) Draw() {
	fb := video.Framebuffer()
	if fb == nil {
		return
	}
	bounds := fb.Bounds()

	height := 0
	b := v.buf.Bytes()
	bb := b
	lines := b[:0]
	maxLines := bounds.Dy() / int(font.Height)
	lineCnt := 0
	skipped := 0
	for height < bounds.Dy() {
		lineCnt++

		idx := bytes.LastIndexByte(bb, '\n')
		if idx == -1 {
			lines = b
			break
		}
		bb, lines = b[:idx], b[idx:]

		if skipped < v.scroll.Y {
			skipped++
		} else {
			height += int(font.Height)
		}
	}

	v.scroll.Y = min(max(0, skipped), lineCnt-maxLines)

	bounds.Min = bounds.Min.Add(v.scroll)
	draw.Src.Draw(fb, fb.Bounds(), image.Black, image.Point{})
	draw.DrawText(fb, bounds, font, image.Point{X: v.scroll.X}, image.White, image.Black, lines)
}
