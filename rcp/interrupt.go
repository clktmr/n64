package rcp

import (
	"n64/rcp/rdp"
	"n64/rcp/serial"
	"n64/rcp/video"

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
