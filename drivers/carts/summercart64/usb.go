package summercart64

import (
	"embedded/rtos"
	"errors"
	"time"
)

// Write writes data from p to the USB port.
func (v *Cart) Write(p []byte) (n int, err error) {
	for len(p) > 0 {
		err = waitUSB(cmdUSBWriteStatus)
		if err != nil {
			return
		}

		var nn int
		nn, err = usbBuf.WriteAt(p[:min(len(p), usbBuf.Size())], 0)
		if err != nil {
			return
		}
		p = p[nn:]

		datatype := 1
		header := uint32(((datatype) << 24) | ((nn) & 0x00FFFFFF))
		_, _, err = execCommand(cmdUSBWrite, uint32(usbBuf.Addr()), header)
		if err != nil {
			return
		}

		n += nn
	}

	return
}

// Read reads pending data from the USB port into p.
func (v *Cart) Read(p []byte) (n int, err error) {
	var length uint32
	for usb.Wait(-1) {
		_, length, err = execCommand(cmdUSBReadStatus, 0, 0)
		if err != nil {
			return
		}
		if length > 0 {
			break
		}
	}

	pending := min(len(p), int(length), usbBuf.Size())
	_, _, err = execCommand(cmdUSBRead, uint32(usbBuf.Addr()), uint32(pending))
	if err != nil {
		return
	}

	err = waitUSB(cmdUSBReadStatus)
	if err != nil {
		return
	}

	n, err = usbBuf.ReadAt(p[:pending], 0)

	// sc64 adds null terminator as EOL, replace with newline
	if p[n-1] == 0 {
		p[n-1] = '\n'
	}

	return
}

func waitUSB(cmd command) error {
	start := rtos.Nanotime()
	for {
		status, _, err := execCommand(cmd, 0, 0)
		if err != nil {
			return err
		}
		if status&uint32(statusBusy) == 0 {
			break
		}
		if rtos.Nanotime()-start > time.Second {
			return errors.New("usb timeout")
		}
	}
	return nil
}

func execCommand(cmdId command, data0 uint32, data1 uint32) (result0 uint32, result1 uint32, err error) {
	regs().data0.Store(data0)
	regs().data1.Store(data1)
	regs().status.Store(status(cmdId) | statusCmdIrqRequest)

	if !cmd.Wait(1 * time.Second) {
		err = errors.New("command timeout")
		return
	}

	result0 = regs().data0.Load()
	result1 = regs().data1.Load()

	if regs().status.Load()&statusError != 0 {
		err = Error(result0)
	}

	return
}
