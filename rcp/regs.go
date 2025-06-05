package rcp

import (
	"embedded/mmio"

	"github.com/clktmr/n64/rcp/cpu"
)

// The RCP's clock speed
const ClockSpeed = 62.5e6

var regs *registers = cpu.MMIO[registers](0x0430_0000)

// The RCP has multiple interrupts, which are all routed to the same external
// interrupt line on the CPU. So all of these must be handled in the
// IRQ3_Handler.
type interruptFlag uint32

const (
	IntrRSP    interruptFlag = 1 << iota // RSP breakpoint or software interrupt
	IntrSerial                           // SI DMA to/from PIF RAM finished
	IntrAudio                            // playback of audio buffer started
	IntrVideo                            // VBlank, line configurable with video.regs.vInt
	IntrPeriph                           // PI bus DMA tranfer finished
	IntrRDP                              // RDP full sync (see FULL_SYNC command)

	IntrLast
)

type modeFlag uint32

const RepeatCountMask modeFlag = 0x7f

// mode read access
const (
	Repeat modeFlag = 1 << (iota + 7)
	EBus
	Upper
)

// mode write access
const (
	ClearRepeat modeFlag = 1 << (iota + 7)
	SetRepeat
	ClearEBus
	SetEBus
	ClearDP
	ClearUpper
	SetUpper
)

type registers struct {
	mode mmio.R32[modeFlag]

	rspVersion mmio.U8
	rdpVersion mmio.U8
	racVersion mmio.U8
	ioVersion  mmio.U8

	// Read-only register with pending interrupts
	interrupt mmio.R32[interruptFlag]

	// When writing to this register, the bits have another meaning: Each
	// interrupt has two bits:
	// 0 - clear SP
	// 1 - set SP
	// 2 - clear SI
	// 3 - set SI
	// ... and so on.
	mask mmio.R32[interruptFlag]
}

func EnableInterrupts(mask interruptFlag) {
	mask = convertMask(mask)
	mask = mask << 1
	regs.mask.Store(mask)
}

func DisableInterrupts(mask interruptFlag) {
	mask = convertMask(mask)
	regs.mask.Store(mask)
}

func Interrupts() {
	regs.mask.Load()
}

func ClearDPIntr() { regs.mode.Store(ClearDP) }

func convertMask(mask interruptFlag) interruptFlag {
	var wmask interruptFlag
	for i := IntrRSP; i < IntrLast; i = i << 1 {
		if mask&i != 0 {
			wmask |= i * i
		}
	}
	return wmask
}
