package rcp

import (
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

var handlers = [6]func(){}

func init() {
	DisableInterrupts(^InterruptFlag(0))
}

//go:linkname rcpHandler IRQ3_Handler
//go:interrupthandler
func rcpHandler() {
	pending := regs.interrupt.Load()
	mask := regs.mask.Load()
	irq := 0
	for flag := InterruptFlag(1); flag != InterruptFlagLast; flag = flag << 1 {
		if flag&pending != 0 && flag&mask != 0 {
			handler := handlers[irq]
			if handler == nil {
				panic("unhandled interrupt")
			}
			handler()
		}
		irq += 1
	}
}

func SetHandler(int InterruptFlag, handler func()) {
	en, prio, _ := RCP.Status(0)
	RCP.Disable(0)

	irq := 0
	for flag := InterruptFlag(1); flag != InterruptFlagLast; flag = flag << 1 {
		if flag&int != 0 {
			handlers[irq] = handler
			break
		}
		irq += 1
	}

	if en {
		RCP.Enable(prio, 0)
	}
}
