package rcp

import (
	"embedded/mmio"
	"unsafe"

	"github.com/clktmr/n64/rcp/cpu"
)

var regs *registers = (*registers)(unsafe.Pointer(baseAddr))

const baseAddr = uintptr(cpu.KSEG1 | 0x0430_0000)

// The RCP has multiple interrupts, which are all routed to the same external
// interrupt line on the CPU.  So all of these must be handled in the
// IRQ3_Handler.
type InterruptFlag uint32

const (
	SignalProcessor     InterruptFlag = 1 << iota // RSP breakpoint or software interrupt
	SerialInterface                               // SI DMA to/from PIF RAM finished
	AudioInterface                                // playback of audio buffer started
	VideoInterface                                // VBlank, line configurable with video.regs.vInt
	PeripheralInterface                           // PI bus DMA tranfer finished
	DisplayProcessor                              // RDP full sync (see FULL_SYNC command)

	InterruptFlagLast
)

type ModeFlag uint32

const RepeatCountMask ModeFlag = 0x7f

// mode read access
const (
	Repeat ModeFlag = 1 << (iota + 7)
	EBus
	Upper
)

// mode write access
const (
	ClearRepeat ModeFlag = 1 << (iota + 7)
	SetRepeat
	ClearEBus
	SetEBus
	ClearDP
	ClearUpper
	SetUpper
)

type registers struct {
	mode mmio.R32[ModeFlag]

	rspVersion mmio.U8
	rdpVersion mmio.U8
	racVersion mmio.U8
	ioVersion  mmio.U8

	// Read-only register with pending interrupts
	interrupt mmio.R32[InterruptFlag]

	// When writing to this register, the bits have another meaning:  Each
	// interrupt has two bits:
	// 0 - clear SP
	// 1 - set SP
	// 2 - clear SI
	// 3 - set SI
	// ... and so on.
	mask mmio.R32[InterruptFlag]
}

func EnableInterrupts(mask InterruptFlag) {
	mask = convertMask(mask)
	mask = mask << 1
	regs.mask.Store(mask)
}

func DisableInterrupts(mask InterruptFlag) {
	mask = convertMask(mask)
	regs.mask.Store(mask)
}

func convertMask(mask InterruptFlag) InterruptFlag {
	var wmask InterruptFlag
	for i := SignalProcessor; i < InterruptFlagLast; i = i << 1 {
		if mask&i != 0 {
			wmask |= i * i
		}
	}
	return wmask
}
