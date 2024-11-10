package display

import (
	"embedded/rtos"
	"image"
	"time"

	"github.com/clktmr/n64/rcp"
	"github.com/clktmr/n64/rcp/rdp"
	"github.com/clktmr/n64/rcp/texture"
	"github.com/clktmr/n64/rcp/video"
)

// Display implements a vsynced, double buffered framebuffer.
type Display struct {
	read, write texture.Texture
	start       time.Time
	vsync       *rtos.Note

	rendertime, frametime time.Duration
	cmd, pipe, tmem       uint32
}

func NewDisplay(resolution image.Point, bpp video.ColorDepth, vsync *rtos.Note) *Display {
	fb := &Display{vsync: vsync}

	bounds := image.Rectangle{Max: resolution}
	if bpp == video.BPP16 {
		fb.read = texture.NewRGBA16(bounds)
		fb.write = texture.NewRGBA16(bounds)
	} else if bpp == video.BPP32 {
		fb.read = texture.NewRGBA32(bounds)
		fb.write = texture.NewRGBA32(bounds)
	}

	video.SetupPAL(fb.read) // TODO

	fb.start = time.Now()

	return fb
}

// Returns the next framebuffer for rendering.  The framebuffer returned by the
// last call becomes invalid.  Blocks until vblank if vsync is enabled.
func (p *Display) Swap() texture.Texture {
	p.rendertime = time.Since(p.start)
	p.cmd, p.pipe, p.tmem = rdp.Busy()

	if p.vsync != nil {
		video.VBlank.Clear()
		if !video.VBlank.Sleep(1 * time.Second) {
			panic("vblank timeout")
		}
	}

	p.frametime = time.Since(p.start)
	p.start = time.Now()
	p.read, p.write = p.write, p.read

	video.SetFrambuffer(p.read)
	return p.write
}

func (p *Display) FPS() float32 {
	return 1e9 / float32(p.frametime)
}

func (p *Display) Duration() time.Duration {
	return p.rendertime
}

func (p *Display) Utilization() (cmd, pipe, tmem time.Duration) {
	cmd = time.Duration(float32(p.cmd) * (1e9 / rcp.ClockSpeed))
	pipe = time.Duration(float32(p.pipe) * (1e9 / rcp.ClockSpeed))
	tmem = time.Duration(float32(p.tmem) * (1e9 / rcp.ClockSpeed))
	return
}
