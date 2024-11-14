package video

import (
	"embedded/rtos"
	"sync/atomic"

	"github.com/clktmr/n64/rcp/cpu"
	"github.com/clktmr/n64/rcp/texture"
)

var VBlank rtos.Note
var VBlankCnt uint

var Odd atomic.Bool

//go:nosplit
//go:nowritebarrierrec
func Handler() {
	line := regs.vCurrent.Load()
	regs.vCurrent.Store(line) // clears interrupt

	Odd.Store(line&1 == 0)

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
	if Interlaced {
		if Odd.Load() {
			offset := 1024 * fb.Bounds().Dy() / NativeResolution().Y
			if offset < 1024 {
				setVerticalOffset(offset)
			} else { // corner case @ native vertical resolution
				addr += uint32(texture.PixelsToBytes(fb.Stride(), fb.BPP()))
			}
		} else {
			setVerticalOffset(0)
		}
	}
	regs.origin.Store(addr)
}

// Shifts the framebuffer vertically by a fraction of a pixel.  A maximum value
// of 1023 is accepted, where 1024 equals one pixel.
//
//go:nosplit
func setVerticalOffset(subpixel int) {
	regs.yScale.Store(uint32(subpixel)<<16 | 0xffff&regs.yScale.Load())
}
