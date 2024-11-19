package video

import (
	"embedded/rtos"
	"image"
	"sync/atomic"

	"github.com/clktmr/n64/rcp/cpu"
	"github.com/clktmr/n64/rcp/texture"
)

var VBlank rtos.Note
var VBlankCnt uint

var Odd atomic.Bool

// The handler is guaranteed to never be called with a nil framebuffer.
//
//go:nosplit
//go:nowritebarrierrec
func Handler() {
	line := regs.vCurrent.Load()
	regs.vCurrent.Store(line) // clears interrupt

	Odd.Store(line&1 == 0)

	// update scale if it was changed
	h := regs.hVideo.Load()
	v := regs.vVideo.Load()
	currentScale := image.Rectangle{
		image.Point{int(h >> 16), int(v >> 16)},
		image.Point{int(h & 0xffff), int(v & 0xffff)},
	}
	if r := scale.Load(); *r != currentScale {
		fbSize := framebuffer.Bounds().Size()
		videoSize := r.Size()
		regs.hVideo.Store(uint32(r.Min.X<<16 | r.Max.X))
		regs.vVideo.Store(uint32(r.Min.Y<<16 | r.Max.Y))
		regs.xScale.Store(uint32((fbSize.X<<10 + videoSize.X>>1) / (videoSize.X)))
		regs.yScale.Store(uint32((fbSize.Y<<10 + videoSize.Y>>2) / (videoSize.Y >> 1)))
	}

	updateFramebuffer()

	VBlankCnt += 1
	VBlank.Wakeup()
}

// Updates the framebuffer based on currently configured framebuffer and field.
//
//go:nosplit
func updateFramebuffer() {
	fb := framebuffer
	addr := cpu.PhysicalAddress(fb.Addr())
	if interlaced {
		// Shift the framebuffer vertically based on current field.
		yScale := regs.yScale.Load()
		if Odd.Load() {
			yOffset := int(0xffff&yScale) >> 1
			// Move framebuffer address by a whole line if offset is
			// more than a pixel.
			for yOffset >= 1024 {
				yOffset -= 1024
				addr += uint32(texture.PixelsToBytes(fb.Stride(), fb.BPP()))
			}
			yScale = (uint32(yOffset)<<16 | 0xffff&regs.yScale.Load())
		} else {
			yScale = (0xffff & regs.yScale.Load())
		}
		regs.yScale.Store(yScale)
	}
	regs.origin.Store(addr)
}
