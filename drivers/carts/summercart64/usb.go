package summercart64

import (
	"io"
)

func (v *SummerCart64) Write(p []byte) (n int, err error) {
	writeEnable, err := v.SetConfig(CfgROMWriteEnable, 1)
	if err != nil {
		return 0, err
	}

	for errShort := io.ErrShortWrite; errShort == io.ErrShortWrite; {
		_, err = usbBuf.Seek(0, io.SeekStart)
		if err != nil {
			return 0, err
		}
		n, errShort = usbBuf.Write(p)
		p = p[n:]
		usbBuf.Flush()

		datatype := 1
		header := uint32(((datatype) << 24) | ((n) & 0x00FFFFFF))
		_, _, err = execCommand(cmdUSBWrite, uint32(usbBuf.Addr()), header)
		if err != nil {
			return 0, err
		}

		err = waitUSB(cmdUSBWriteStatus)
		if err != nil {
			return 0, err
		}
	}

	_, err = v.SetConfig(CfgROMWriteEnable, writeEnable)
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

	usbBuf.WritebackInvalidate()

	pending := min(len(p), int(length), bufferSize)
	_, _, err = execCommand(cmdUSBRead, uint32(usbBuf.Addr()), uint32(pending))
	if err != nil {
		return 0, err
	}

	err = waitUSB(cmdUSBReadStatus)
	if err != nil {
		return 0, err
	}

	usbBuf.Seek(0, io.SeekStart)
	n, err1 := usbBuf.Read(p[:pending])

	_, err = v.SetConfig(CfgROMWriteEnable, writeEnable)
	if err != nil {
		return 0, err
	}

	// sc64 adds null terminator as EOL, replace with newline
	if p[n-1] == 0 {
		p[n-1] = '\n'
	}

	return n, err1
}

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

	status := statusBusy
	for status&statusBusy != 0 {
		status = regs.status.Load()
	}

	result0 = regs.data0.Load()
	result1 = regs.data1.Load()

	if status&statusError != 0 {
		err = errCodes[result0]
	}

	return
}
