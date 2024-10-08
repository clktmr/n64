package serial

import (
	"embedded/rtos"
	"io"
	"sync"
	"unsafe"

	"github.com/clktmr/n64/rcp/cpu"
)

type pifCommand byte

const (
	CmdConfigureJoybus pifCommand = 0x1 << iota
	CmdCICChallenge
	_
	CmdTerminateBoot
	CmdLockROM
	CmdAcquireChecksum
	CmdRunChecksum
)

// state shared with interrupt handler
var (
	cmdFinished rtos.Note
	buf         []byte // FIXME atomic
	mtx         sync.Mutex
)

type CommandBlock struct {
	cmd pifCommand
	buf []byte
}

func NewCommandBlock(cmd pifCommand) *CommandBlock {
	buf := cpu.MakePaddedSlice[byte](pifRamSize)[:0]
	return &CommandBlock{cmd, buf}
}

func (c *CommandBlock) Alloc(n int) ([]byte, error) {
	if n > c.Free() {
		return nil, io.EOF
	}
	l := len(c.buf)
	c.buf = c.buf[:l+n]
	return c.buf[l:], nil
}

func (c *CommandBlock) Free() int {
	return cap(c.buf) - len(c.buf) - 1 // save one byte for PIF command
}

func Run(block *CommandBlock) {
	mtx.Lock()
	defer func() { mtx.Unlock() }()

	buf = block.buf[:pifRamSize]
	buf[len(buf)-1] = byte(block.cmd)

	sendAddr := uintptr(unsafe.Pointer(unsafe.SliceData(buf)))

	cmdFinished.Clear()
	cpu.WritebackSlice(buf)
	regs.dramAddr.Store(uint32(sendAddr))
	regs.pifWriteAddr.Store(pifRamAddr)

	// Wait until message was received
	cmdFinished.Sleep(-1) // TODO sleep with timeout

	buf = nil
}

//go:nosplit
//go:nowritebarrierrec
func Handler() {
	regs.status.Store(0) // clears interrupt

	if buf == nil {
		return
	}

	if buf[0x3f] == 0x00 {
		// DMA read finished
		cmdFinished.Wakeup()
	} else {
		// DMA write finished, trigger read back
		recvAddr := uintptr(unsafe.Pointer(unsafe.SliceData(buf)))

		cpu.InvalidateSlice(buf)
		regs.dramAddr.Store(uint32(recvAddr))
		regs.pifReadAddr.Store(pifRamAddr)
	}
}
