package video

import (
	"embedded/rtos"
	"image"

	"github.com/clktmr/n64/rcp"
	"github.com/clktmr/n64/rcp/cpu"
	"github.com/clktmr/n64/rcp/texture"
)

// VBlank can be used to wait until the next vertical blank.
var VBlank rtos.Cond

// Consumed by interrupt handler
var (
	framebuffer rcp.IntrInput[texture.Texture]
	scale       rcp.IntrInput[image.Rectangle]
)

func init() {
	rcp.SetHandler(rcp.IntrVideo, handler)
	rcp.EnableInterrupts(rcp.IntrVideo)
}

// The handler is guaranteed to never be called with a nil framebuffer.
//
//go:nosplit
//go:nowritebarrierrec
func handler() {
	regs.vCurrent.Store(0) // clears interrupt

	fb, _ := framebuffer.Get()
	if fb == nil { // only needed for Ares
		return
	}

	// update scale if it was changed
	if r, updated := scale.Get(); updated {
		fbSize := fb.Bounds().Size()
		videoSize := r.Size()
		regs.hVideo.Store(uint32(r.Min.X<<16 | r.Max.X))
		regs.vVideo.Store(uint32(r.Min.Y<<16 | r.Max.Y))
		regs.xScale.Store(uint32((fbSize.X<<10 + videoSize.X>>1) / (videoSize.X)))
		regs.yScale.Store(uint32((fbSize.Y<<10 + videoSize.Y>>2) / (videoSize.Y >> 1)))
	}

	updateFramebuffer(fb)

	VBlank.Signal()
}

// Updates the framebuffer based on currently configured framebuffer and field.
//
//go:nosplit
func updateFramebuffer(fb texture.Texture) {
	addr := fb.Addr()
	if regs.control.Load()&uint32(controlSerrate) != 0 {
		// Shift the framebuffer vertically based on current field.
		yScale := regs.yScale.Load()
		if regs.vCurrent.Load()&1 == 0 { // odd field
			yOffset := int(0xffff&yScale) >> 1
			// Move framebuffer address by a whole line if offset is
			// more than a pixel.
			for yOffset >= 1024 {
				yOffset -= 1024
				addr += cpu.Addr(fb.BPP().Bytes(fb.Stride()))
			}
			yScale = (uint32(yOffset)<<16 | 0xffff&regs.yScale.Load())
		} else { // even field
			yScale = (0xffff & regs.yScale.Load())
		}
		regs.yScale.Store(yScale)
	}
	regs.origin.Store(addr)
}
