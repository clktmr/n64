package video

import "embedded/rtos"

var VBlank rtos.Note
var VBlankCnt uint

//go:nosplit
//go:nowritebarrierrec
func Handler() {
	line := regs.vCurrent.Load()
	regs.vCurrent.Store(line) // clears interrupt

	VBlankCnt += 1
	VBlank.Wakeup()
}
