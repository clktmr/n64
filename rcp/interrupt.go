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
	RCP      rtos.IRQ = 3 // RCP forwards interrupt by another peripheral
	CART     rtos.IRQ = 4 // Interrupt caused by a peripheral on the cartridge
	PRENMI   rtos.IRQ = 5 // User has pushd reset button on console
	RDBREAD  rtos.IRQ = 6 // Devboard has read the value in the RDB port
	RDBWRITE rtos.IRQ = 7 // Devboard has written a value in the RDB port
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
		regs.mode.Store(ClearDP)
		rdp.Handler()
	default:
		panic("unknown rcp interrupt")
	}
}
