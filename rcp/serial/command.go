package serial

import (
	"embedded/rtos"
	"io"
	"sync"
	"time"

	"github.com/clktmr/n64/rcp"
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

var mtx sync.Mutex

// state shared with interrupt handler
var (
	cmdFinished rtos.Cond
	cmdBuffer   rcp.IntrInput[[]byte]
)

func init() {
	rcp.SetHandler(rcp.IntrSerial, handler)
	rcp.EnableInterrupts(rcp.IntrSerial)
}

//go:nosplit
//go:nowritebarrierrec
func handler() {
	regs.status.Store(0) // clears interrupt

	buf, _ := cmdBuffer.Get()
	if buf == nil {
		return
	}

	if buf[pifRamSize-1] == 0x00 {
		// DMA read finished
		cmdFinished.Signal()
	} else {
		// DMA write finished, trigger read back
		cpu.InvalidateSlice(buf)
		regs.dramAddr.Store(cpu.PhysicalAddressSlice(buf))
		regs.pifReadAddr.Store(pifRamAddr)
	}
}

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
	defer mtx.Unlock()

	buf := block.buf[:pifRamSize]
	buf[len(buf)-1] = byte(block.cmd)

	cmdBuffer.Put(buf)
	cpu.WritebackSlice(buf)
	regs.dramAddr.Store(cpu.PhysicalAddressSlice(buf))
	regs.pifWriteAddr.Store(pifRamAddr)

	// Wait until message was received
	if !cmdFinished.Wait(1 * time.Second) {
		panic("pif timeout")
	}

	cmdBuffer.Put(nil)
}
