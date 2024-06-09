package framebuffer

import (
	"image"
	"image/draw"
	"unsafe"

	"github.com/clktmr/n64/rcp/video"
)

const (
	WIDTH  = 320
	HEIGHT = 240
)

// Represents an image that the DAC can read and output on a screen. Implements
// draw.Image, so all the drawing tools from the standard library can be used.
// Moreover draw.DrawMask will chose optimized implementations based on type
// assertions.  Thats why it's important to be a image.RGBA specifically,  a
// type that the draw package knows. Still all rendering done this way is
// withoug hardware acceleration and rather slow.
// TODO support other resolutions
type Framebuffer struct {
	bufs        [2]draw.Image
	read, write draw.Image
	fill        image.Uniform
}

func NewFramebuffer(bbp video.ColorDepth) *Framebuffer {
	fb := &Framebuffer{}

	for i := range fb.bufs {
		if bbp == video.BBP16 {
			fb.bufs[i] = NewRGBA16(image.Rect(0, 0, WIDTH, HEIGHT))
		} else if bbp == video.BBP32 {
			fb.bufs[i] = image.NewRGBA(image.Rect(0, 0, WIDTH, HEIGHT))
		}
	}

	fb.write = fb.bufs[0]
	fb.read = fb.bufs[1]
	return fb
}

func (fb *Framebuffer) Swap() uintptr {
	tmp := fb.read
	fb.read = fb.write
	fb.write = tmp
	switch buf := fb.write.(type) {
	case *RGBA16:
		return uintptr(unsafe.Pointer(&buf.Pix[:1][0]))
	case *image.RGBA:
		return uintptr(unsafe.Pointer(&buf.Pix[:1][0]))
	}
	return 0x0
}

func (fb *Framebuffer) Bounds() image.Rectangle {
	return fb.write.Bounds()
}

func (fb *Framebuffer) Addr() uintptr {
	switch buf := fb.write.(type) {
	case *RGBA16:
		return uintptr(unsafe.Pointer(&buf.Pix[:1][0]))
	case *image.RGBA:
		return uintptr(unsafe.Pointer(&buf.Pix[:1][0]))
	}
	return 0x0
}
