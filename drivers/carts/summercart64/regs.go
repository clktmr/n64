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
	identifier periph.U32Safe
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

type SummerCart64 struct {
	saveStorage periph.Device
}

func Probe() *SummerCart64 {
	// sc64 magic unlock sequence
	regs.key.Store(0x0)
	regs.key.Store(0x5f554e4c)
	regs.key.Store(0x4f434b5f)

	if regs.identifier.Load() == 0x53437632 { // SummerCart64 V2
		s := &SummerCart64{}
		if st, err := s.Config(CfgSaveType); err == nil {
			params := saveStorageParams[st]
			s.saveStorage = *periph.NewDevice(params.addr, params.size)
		}

		_, _ = s.SetConfig(CfgROMWriteEnable, 1)

		return s
	}
	return nil
}

// Returns the current storage for save files, configured by savetype.  Returns
// a device with Size==0 if no savetype is configured.
// FIXME shouldn't be here, instead have a generic Probe function to get
// storage.  Otherwise we could get multiple periph.Devices pointing to the same
// address range, messing up the caching.
func (v *SummerCart64) SaveStorage() *periph.Device {
	// FIXME no writeback triggered for EEPROM savetypes
	return &v.saveStorage
}

//go:nosplit
func (v *SummerCart64) ClearInterrupt() {
	regs.identifier.Store(0)
}
