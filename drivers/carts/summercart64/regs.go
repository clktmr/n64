package summercart64

import (
	"unsafe"

	"github.com/clktmr/n64/rcp/cpu"
	"github.com/clktmr/n64/rcp/periph"
)

const baseAddr uintptr = cpu.KSEG1 | 0x1fff_0000
const bufferSize = 512

// It's up to us to choose a location in the ROM.  This puts it at the end of a
// 64MB cartridge.
var usbBuf = periph.NewDevice(0x1400_0000-bufferSize, bufferSize)

type registers struct {
	status     periph.R32[status]
	data0      periph.U32
	data1      periph.U32
	identifier periph.U32
	key        periph.U32
}

var regs *registers = (*registers)(unsafe.Pointer(baseAddr))

type status uint32

const (
	statusBusy       status = 1 << 31
	statusError      status = 1 << 30
	statusIrqPending status = 1 << 29
	statusCmdIdMask  status = 0xff
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

type SummerCart64 struct{}

func Probe() *SummerCart64 {
	// sc64 magic unlock sequence
	regs.key.Store(0x0)
	regs.key.Store(0x5f554e4c)
	regs.key.Store(0x4f434b5f)

	if regs.identifier.Load() == 0x53437632 { // SummerCart64 V2
		return &SummerCart64{}
	}
	return nil
}

//go:nosplit
func (v *SummerCart64) ClearInterrupt() {
	regs.identifier.Store(0)
}
