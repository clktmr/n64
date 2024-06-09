package rcp

import (
	"github.com/clktmr/n64/rcp/rdp"
	"github.com/clktmr/n64/rcp/rsp"
	"github.com/clktmr/n64/rcp/serial"
	"github.com/clktmr/n64/rcp/video"

	"embedded/rtos"

	_ "unsafe" // for linkname
)

const (
	RCP    rtos.IRQ = 2
	CART   rtos.IRQ = 3
	PRENMI rtos.IRQ = 4
)

//go:linkname handler IRQ3_Handler
//go:interrupthandler
func handler() {
	pending := regs.interrupt.Load()
	switch {
	case pending&SignalProcessor != 0:
		rsp.Handler()
	case pending&VideoInterface != 0:
		video.Handler()
	case pending&SerialInterface != 0:
		serial.Handler()
	case pending&DisplayProcessor != 0:
		regs.mode.SetBits(0x800) // TODO name const
		rdp.Handler()
	default:
		panic("unknown rcp interrupt")
	}
}
