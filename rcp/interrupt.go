package rcp

import (
	"embedded/rtos"

	_ "unsafe" // for linkname
)

const (
	IrqRcp      rtos.IRQ = 3 // RCP forwards an interrupt by another peripheral
	IrqCart     rtos.IRQ = 4 // Interrupt caused by a peripheral on the cartridge
	IrqPrenmi   rtos.IRQ = 5 // User has pushed reset button on console
	IrqRdbRead  rtos.IRQ = 6 // Devboard has read the value in the RDB port
	IrqRdbWrite rtos.IRQ = 7 // Devboard has written a value in the RDB port
)

var handlers = [6]func(){}

func init() {
	DisableInterrupts(^interruptFlag(0))
	IrqRcp.Enable(rtos.IntPrioLow, 0)
	IrqPrenmi.Enable(rtos.IntPrioLow, 0)
}

//go:linkname rcpHandler IRQ3_Handler
//go:interrupthandler
func rcpHandler() {
	pending := regs().interrupt.Load()
	mask := regs().mask.Load()
	irq := 0
	for flag := interruptFlag(1); flag != IntrLast; flag = flag << 1 {
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

func SetHandler(int interruptFlag, handler func()) {
	en, prio, _ := IrqRcp.Status(0)
	IrqRcp.Disable(0)

	irq := 0
	for flag := interruptFlag(1); flag != IntrLast; flag = flag << 1 {
		if flag&int != 0 {
			handlers[irq] = handler
			break
		}
		irq += 1
	}

	if en {
		IrqRcp.Enable(prio, 0)
	}
}

func Handler(int interruptFlag) func() {
	irq := 0
	for flag := interruptFlag(1); flag != IntrLast; flag = flag << 1 {
		if flag&int != 0 {
			return handlers[irq]
		}
		irq += 1
	}
	return nil
}

// Reset signals that the console's reset button was pressed. The hardware
// reboots with the button's release, but not before 500ms have passed.
var Reset rtos.Cond

//go:linkname prenmiHandler IRQ5_Handler
//go:interrupthandler
func prenmiHandler() {
	IrqPrenmi.Disable(0)
	Reset.Signal()
}
