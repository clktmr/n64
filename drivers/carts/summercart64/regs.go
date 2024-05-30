package summercart64

import (
	"errors"
	"io"
	"n64/rcp/cpu"
	"n64/rcp/periph"
	"unsafe"
)

var regs *registers = (*registers)(unsafe.Pointer(baseAddr))

const baseAddr = uintptr(cpu.KSEG1 | 0x1fff_0000)
const bufferSize = 512

// It's up to us to choose a location in the ROM.  This puts it at the end of a
// 64MB cartridge.
var usbBuf *[128]periph.U32 = (*[128]periph.U32)(unsafe.Pointer(uintptr(cpu.KSEG1 | (0x1400_0000 - bufferSize))))

type registers struct {
	status     periph.R32[status]
	data0      periph.U32
	data1      periph.U32
	identifier periph.U32
	key        periph.U32
}

type status uint32

const (
	statusBusy       status = 1 << 31
	statusError      status = 1 << 30
	statusIrqPending status = 1 << 29
	statusCmdIdMask  status = 0xff
)

type command status

const (
	cmdConfigGet       = 'c'
	cmdConfigSet       = 'C'
	cmdUSBRead         = 'm'
	cmdUSBWrite        = 'M'
	cmdUSBReadStatus   = 'u'
	cmdUSBWrtiteStatus = 'U'
)

type config uint32

const (
	cfgBootloaderSwitch = iota
	cfgROMWriteEnable
	cfgROMShadowEnable
	cfgDDMode
	cfgISVAddress
	cfgBootMode
	cfgSaveType
	cfgCICSeed
	cfgTVType
	cfgDDSDEnable
	cfgDDDriveType
	cfgDDDiskState
	cfgButtonState
	cfgButtonMode
	cfgROMExtendedEnable
)

type SummerCart64 struct {
	buf []byte
}

func Probe() *SummerCart64 {
	// sc64 magic unlock sequence
	regs.key.Store(0x0)
	regs.key.Store(0x5f554e4c)
	regs.key.Store(0x4f434b5f)

	if regs.identifier.Load() == 0x53437632 { // SummerCart64 V2
		return &SummerCart64{
			buf: cpu.MakePaddedSlice(bufferSize),
		}
	}
	return nil
}

func (v *SummerCart64) Write(p []byte) (n int, err error) {
	_, writeEnable, err := execCommand(cmdConfigSet, cfgROMWriteEnable, 1)
	if err != nil {
		return 0, err
	}

	n = len(p)
	if n > bufferSize {
		n = bufferSize
		err = io.ErrShortWrite
	}

	// If used as a SystemWriter we might be in a syscall.  Make sure we
	// don't allocate in DMAStore, or we might panic with "malloc during
	// signal".
	if cpu.IsPadded(p) == false {
		copy(v.buf, p)
		p = v.buf
	}

	periph.DMAStore(usbBuf[0].Addr(), p[:n+n%2])

	_, _, err = execCommand(cmdConfigSet, cfgROMWriteEnable, writeEnable)
	if err != nil {
		return 0, err
	}

	datatype := 1
	header := uint32(((datatype) << 24) | ((n) & 0x00FFFFFF))
	_, _, err = execCommand(cmdUSBWrite, uint32(usbBuf[0].Addr()), header)
	if err != nil {
		return 0, err
	}

	err = waitUSBBusy()
	if err != nil {
		return 0, err
	}

	return n, err
}

var ErrCommand error = errors.New("execute sc64 command")

func waitUSBBusy() error {
	for {
		status, _, err := execCommand(cmdUSBWrtiteStatus, 0, 0)
		if err != nil {
			return err
		}
		if status != uint32(statusBusy) {
			break
		}
	}
	return nil
}

func execCommand(cmdId command, data0 uint32, data1 uint32) (result0 uint32, result1 uint32, err error) {
	regs.data0.Store(data0)
	regs.data1.Store(data1)
	regs.status.Store(status(cmdId))

	for {
		if regs.status.Load()&statusBusy == 0 {
			break
		}
	}

	if regs.status.Load()&statusError != 0 {
		return 0, 0, ErrCommand
	}

	result0 = regs.data0.Load()
	result1 = regs.data1.Load()
	return
}
