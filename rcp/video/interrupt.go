package video

import "embedded/rtos"

// var vBlank rtos.Note
var VBlank rtos.Note
var VBlankCnt uint

// TODO go:nosplit ??
func Handler() {
	line := regs.currentLine.Load()
	regs.currentLine.Store(line) // clears interrupt

	VBlankCnt += 1
	VBlank.Wakeup()
}
