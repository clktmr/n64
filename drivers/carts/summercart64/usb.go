package summercart64

import (
	"errors"
	"io"

	"github.com/clktmr/n64/rcp/cpu"
	"github.com/clktmr/n64/rcp/periph"
)

func (v *SummerCart64) Write(p []byte) (n int, err error) {
	writeEnable, err := v.SetConfig(CfgROMWriteEnable, 1)
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

	_, err = v.SetConfig(CfgROMWriteEnable, writeEnable)
	if err != nil {
		return 0, err
	}

	datatype := 1
	header := uint32(((datatype) << 24) | ((n) & 0x00FFFFFF))
	_, _, err = execCommand(cmdUSBWrite, uint32(usbBuf[0].Addr()), header)
	if err != nil {
		return 0, err
	}

	err = waitUSB(cmdUSBWriteStatus)
	if err != nil {
		return 0, err
	}

	return n, err
}

func (v *SummerCart64) Read(p []byte) (n int, err error) {
	msgtype, length, err := execCommand(cmdUSBReadStatus, 0, 0)
	if msgtype == 0 || err != nil {
		return 0, err
	}

	writeEnable, err := v.SetConfig(CfgROMWriteEnable, 1)
	if err != nil {
		return 0, err
	}

	n = min(len(p), int(length), bufferSize)
	_, _, err = execCommand(cmdUSBRead, uint32(usbBuf[0].Addr()), uint32(n))
	if err != nil {
		return 0, err
	}

	err = waitUSB(cmdUSBReadStatus)
	if err != nil {
		return 0, err
	}

	periph.DMALoad(usbBuf[0].Addr(), p[:n+n%2])

	_, err = v.SetConfig(CfgROMWriteEnable, writeEnable)
	if err != nil {
		return 0, err
	}

	// sc64 adds null terminator as EOL, replace with newline
	if p[n-1] == 0 {
		p[n-1] = '\n'
	}

	return n, err
}

var ErrCommand error = errors.New("execute sc64 command")

func waitUSB(cmd command) error {
	for {
		status, _, err := execCommand(cmd, 0, 0)
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
