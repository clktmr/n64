package rsp

import (
	"embedded/mmio"
	"errors"
	"io"
	"runtime"
	"sync"

	"github.com/clktmr/n64/debug"
	"github.com/clktmr/n64/rcp"
	"github.com/clktmr/n64/rcp/cpu"
)

type Memory cpu.Addr

var dmaMtx sync.Mutex

// ReatAt loads bytes from RSP IMEM/DMEM into RDRAM via DMA
func (m Memory) ReadAt(p []byte, off int64) (n int, err error) {
	return m.dma(p, off, true)
}

// WriteAt stores bytes from RDRAM to RSP IMEM/DMEM via DMA
func (m Memory) WriteAt(p []byte, off int64) (n int, err error) {
	return m.dma(p, off, false)
}

func (m Memory) dma(p []byte, off int64, read bool) (n int, err error) {
	if off < 0 || off > 0x1000 {
		return 0, errors.New("offset out of bounds")
	}

	if len(p) == 0 {
		return
	}

	addr := cpu.Addr(m) + cpu.Addr(off)
	end := cpu.Addr(m) + 0x1000
	n = len(p)
	if n > int(end-addr) {
		n = int(end - addr)
		p = p[:n]
		err = io.EOF
	}

	head, tail := cpu.Pads(p)
	if (tail-head)&0x7 != 0 {
		tail &^= 0x7 // make sure length is 8 byte multiple
	}
	if (addr+cpu.Addr(head))%8 != 0 {
		// pp and addr have different alignment, fallback to mmio
		head = 0
		tail = 0
	}
	pp := p[head:tail]
	addr += cpu.Addr(head)

	debug.Assert(regs.status.LoadBits(halted|dmaBusy) != 0, "rsp: dma busy")

	dmaMtx.Lock()
	defer dmaMtx.Unlock()

	regs.rdramAddr.Store(cpu.PhysicalAddressSlice(pp))
	regs.rspAddr.Store(addr)

	if read {
		if head != tail {
			cpu.InvalidateSlice(pp)
			regs.writeLen.Store(uint32(tail - head - 1))
			waitDMA()
		}
		rcp.ReadIO[*mmio.U32](addr, p[:head])
		rcp.ReadIO[*mmio.U32](addr+cpu.Addr(tail), p[tail:])
	} else {
		rcp.WriteIO[*mmio.U32](addr, p[:head])
		rcp.WriteIO[*mmio.U32](addr+cpu.Addr(tail), p[tail:])
		if head != tail {
			cpu.WritebackSlice(pp)
			regs.readLen.Store(uint32(tail - head - 1))
			waitDMA()
		}
	}

	return
}

// Blocks until DMA has finished.
func waitDMA() {
	for regs.status.Load()&(dmaBusy|ioBusy) != 0 {
		runtime.Gosched()
	}
}
