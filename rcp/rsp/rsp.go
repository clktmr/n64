package rsp

import (
	"embedded/rtos"

	"github.com/clktmr/n64/rcp"
	"github.com/clktmr/n64/rcp/rsp/ucode"
)

func init() {
	regs().status.Store(setHalt | clrSingleStep)
	pc().Store(0x1000)
	rcp.SetHandler(rcp.IntrRSP, handler)
	rcp.EnableInterrupts(rcp.IntrRSP)
}

var IntBreak rtos.Cond

//go:nosplit
//go:nowritebarrierrec
func handler() {
	regs().status.Store(clrIntr)
	IntBreak.Signal()
}

func Load(ucode *ucode.UCode) {
	_, err := IMEM.WriteAt(ucode.Text, 0x0)
	if err != nil {
		panic(err)
	}
	_, err = DMEM.WriteAt(ucode.Data, 0x0)
	if err != nil {
		panic(err)
	}

	pc().Store(ucode.Entry)
}
