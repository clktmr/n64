package rsp

import "embedded/rtos"

func Init() {
	regs.status.Store(setHalt | clrSingleStep)
	pc.Store(0x1000)
}

func InterruptOnBreak(enable bool) {
	if enable {
		regs.status.Store(setIntbreak)
	} else {
		regs.status.Store(clrIntbreak)
	}
}

var IntBreak rtos.Note

//go:nosplit
//go:nowritebarrierrec
func Handler() {
	regs.status.Store(clrIntr)
	IntBreak.Wakeup()
}
