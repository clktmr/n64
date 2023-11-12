package serial

import (
	"embedded/rtos"
	"n64/cpu"
	"unsafe"
)

type message [32]uint16 // uint16 for cache line alignment

var (
	in, out     chan *message
	dmaFinished rtos.Note
)

// TODO remove, only for testing
var SIIntrCnt uint

// Virtual address of the memory mapped PIF RAM
const pifRamAddr uint32 = 0x1fc0_07c0

func StartJoybus() {
	in = make(chan *message)
	out = make(chan *message)
	go joybusPoll()
}

func joybusPoll() {
	for {
		sendMsg := <-out
		sendAddr := uintptr(unsafe.Pointer(sendMsg))

		dmaFinished.Clear()
		cpu.Writeback(sendAddr, len(sendMsg))
		regs.dramAddr.Store(uint32(sendAddr) | cpu.KSEG1)
		regs.pifWriteAddr.Store(pifRamAddr)

		// Wait until message was sent
		dmaFinished.Sleep(-1) // TODO sleep with timeout

		var recvMsg message
		recvAddr := uintptr(unsafe.Pointer(&recvMsg))

		dmaFinished.Clear()
		regs.dramAddr.Store(uint32(recvAddr) | cpu.KSEG1)
		regs.pifReadAddr.Store(pifRamAddr)

		// Wait until message was received
		dmaFinished.Sleep(-1) // TODO sleep with timeout

		cpu.Invalidate(recvAddr, len(recvMsg))
		in <- &recvMsg
	}
}

func Query(req *message) *message {
	out <- req
	return <-in
}

// TODO go:nosplit ??
func Handler() {
	regs.status.Store(0) // clears interrupt
	SIIntrCnt += 1
	dmaFinished.Wakeup()
}
