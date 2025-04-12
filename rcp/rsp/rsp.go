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

var IntBreak rtos.Cond

func init() {
	rcp.SetHandler(rcp.IntrRSP, handler)
	rcp.EnableInterrupts(rcp.IntrRSP)
}

//go:nosplit
//go:nowritebarrierrec
func handler() {
	regs.status.Store(clrIntr)
	IntBreak.Signal()
}
