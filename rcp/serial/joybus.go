package serial

import (
	"embedded/rtos"
	"n64/rcp/cpu"
	"unsafe"
)

type message struct {
	_   cpu.CacheLinePad
	buf [32]uint16 // uint16 for 2 byte alignment needed by DMA
	_   cpu.CacheLinePad
}

var (
	in, out     chan *message
	dmaFinished rtos.Note
)

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
		sendAddr := uintptr(unsafe.Pointer(&sendMsg.buf))

		dmaFinished.Clear()
		cpu.Writeback(sendAddr, len(sendMsg.buf))
		regs.dramAddr.Store(uint32(sendAddr))
		regs.pifWriteAddr.Store(pifRamAddr)

		// Wait until message was sent
		dmaFinished.Sleep(-1) // TODO sleep with timeout

		var recvMsg message
		recvAddr := uintptr(unsafe.Pointer(&recvMsg.buf))

		dmaFinished.Clear()
		regs.dramAddr.Store(uint32(recvAddr))
		regs.pifReadAddr.Store(pifRamAddr)

		// Wait until message was received
		dmaFinished.Sleep(-1) // TODO sleep with timeout

		cpu.Invalidate(recvAddr, len(recvMsg.buf))
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
	dmaFinished.Wakeup()
}
