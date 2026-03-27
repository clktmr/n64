// Package summercart64 implements support for SummerCart64.
//
// See https://summercart64.dev/
package summercart64

import (
	"embedded/rtos"
	"sync/atomic"

	"github.com/clktmr/n64/rcp"
	"github.com/clktmr/n64/rcp/cpu"
	"github.com/clktmr/n64/rcp/periph"

	_ "unsafe"
)

var (
	usbBuf    = periph.NewDevice(0x1ffe_0000, 512)
	sdcardBuf = periph.NewDevice(0x1ffe_0200, 15*512)
)

type registers struct {
	status     periph.R32[status]
	data0      periph.U32
	data1      periph.U32
	identifier periph.U32
	key        periph.U32
	irq        periph.R32[irq]
	aux        periph.U32
}

func regs() *registers { return cpu.MMIO[registers](0x1fff_0000) }

type status uint32

const (
	statusBusy          status = 1 << 31
	statusError         status = 1 << 30
	statusBtnIrqPending status = 1 << 29
	statusBtnIrqMask    status = 1 << 28
	statusCmdIrqPending status = 1 << 27
	statusCmdIrqMask    status = 1 << 26
	statusUSBIrqPending status = 1 << 25
	statusUSBIrqMask    status = 1 << 24
	statusAUXIrqPending status = 1 << 23
	statusAUXIrqMask    status = 1 << 22

	statusCmdIrqRequest status = 1 << 8

	statusCmdIdMask status = 0xff
)

type command status

const (
	cmdIdentifierGet    command = 'v'
	cmdVersionGet       command = 'V'
	cmdConfigGet        command = 'c'
	cmdConfigSet        command = 'C'
	cmdSettingGet       command = 'a'
	cmdSettingSet       command = 'A'
	cmdTimeGet          command = 't'
	cmdTimeSet          command = 'T'
	cmdUSBRead          command = 'm'
	cmdUSBWrite         command = 'M'
	cmdUSBReadStatus    command = 'u'
	cmdUSBWriteStatus   command = 'U'
	cmdSDCardOp         command = 'i'
	cmdSDSectorSet      command = 'I'
	cmdSDRead           command = 's'
	cmdSDWrite          command = 'S'
	cmdDiskMappingSet   command = 'D'
	cmdWritebackPending command = 'w'
	cmdWritebackSDInfo  command = 'W'
	cmdFlashProgram     command = 'K'
	cmdFlashWaitBusy    command = 'p'
	cmdFlashEraseBlock  command = 'P'
	cmdDiagnosticGet    command = '%'
)

type irq uint32

const (
	irqBtnClear irq = 1 << 31
	irqCmdClear irq = 1 << 30
	irqUSBClear irq = 1 << 29
	irqAUXClear irq = 1 << 28

	irqUSBDisable irq = 1 << 11
	irqUSBEnable  irq = 1 << 10
	irqAUXDisable irq = 1 << 9
	irqAUXEnable  irq = 1 << 8
)

const (
	SaveNone = iota
	SaveEEPROM4k
	SaveEEPROM16k
	SaveSRAM
	SaveFlashRAM
	SaveSRAMBanked
	SaveSRAM1m
)

var saveStorageParams = [...]struct {
	addr cpu.Addr
	size uint32
}{
	{0x0800_0000, 0},
	{0x1ffe_2000, 512},
	{0x1ffe_2000, 2048},
	{0x0800_0000, 32 * 1024},
	{0x0800_0000, 128 * 1024},
	{0x0800_0000, 3 * 32 * 1024},
	{0x0800_0000, 128 * 1024},
}

// Cart represents a SummerCart64.
type Cart struct {
	saveStorage periph.Device
}

// Probe initializes and returns the [Cart] if a SummerCart64 was detected.
func Probe() *Cart {
	// sc64 magic unlock sequence
	regs().key.Store(0x0)
	regs().key.Store(0x5f554e4c)
	regs().key.Store(0x4f434b5f)

	if regs().identifier.Load() == 0x53437632 { // SummerCart64 V2
		err := rcp.IrqCart.Enable(rtos.IntPrioMid, 0)
		if err != nil {
			panic(err)
		}
		regs().irq.Store(irqUSBEnable)
		s := &Cart{}
		if st, err := s.Config(CfgSaveType); err == nil {
			params := saveStorageParams[st]
			s.saveStorage = *periph.NewDevice(params.addr, params.size)
		}

		return s
	}
	return nil
}

// Returns the current storage for save files, configured by savetype. Returns a
// device with Size==0 if no savetype is configured.
func (v *Cart) SaveStorage() *periph.Device {
	// FIXME shouldn't be here, instead have a generic Probe function to get
	// storage. Otherwise we could get multiple periph.Devices pointing to
	// the same address range, messing up the caching.
	// FIXME no writeback triggered for EEPROM savetypes
	return &v.saveStorage
}

var (
	cmd, usb   rtos.Cond
	btnHandler atomic.Value
)

// SetBtnHandler sets the callback for the button interrupt. The function will
// be invoked from interrupt context and must not allocate or park the
// goroutine. Use at least the go:nosplit and go:nowritebarrierrec pragmas.
func SetBtnHandler(fn func()) {
	btnHandler.Store(fn)
}

//go:linkname handler IRQ4_Handler
//go:interrupthandler
func handler() {
	status := regs().status.LoadSafe()
	irqClear := irq(0)
	if status&statusBtnIrqPending != 0 {
		irqClear |= irqBtnClear
		if h, ok := btnHandler.Load().(func()); ok {
			h()
		}
	}
	if status&statusCmdIrqPending != 0 {
		cmd.Signal()
		irqClear |= irqCmdClear
	}
	if status&statusUSBIrqPending != 0 {
		usb.Signal()
		irqClear |= irqUSBClear
	}
	if status&statusAUXIrqPending != 0 {
		irqClear |= irqAUXClear
	}
	regs().irq.StoreSafe(irqClear)
}
