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

	if Interlaced {
		addr := cpu.PhysicalAddress(framebuffer.Addr())
		if Interlaced && Odd.Load() {
			addr += uint32(texture.PixelsToBytes(framebuffer.Stride(), framebuffer.BPP()))
		}
		regs.origin.Store(addr)
	}

	VBlankCnt += 1
	VBlank.Wakeup()
}
