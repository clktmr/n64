package rsp

import (
	"embedded/rtos"

	"github.com/clktmr/n64/rcp"
)

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

func init() {
	rcp.SetHandler(rcp.IntrRSP, Handler)
	rcp.EnableInterrupts(rcp.IntrRSP)
}

//go:nosplit
//go:nowritebarrierrec
func Handler() {
	regs.status.Store(clrIntr)
	IntBreak.Wakeup()
}
