package controller

import (
	"io"

	"github.com/clktmr/n64/debug"
	"github.com/clktmr/n64/drivers/controller/pakfs"
	"github.com/clktmr/n64/rcp/serial"
	"github.com/clktmr/n64/rcp/serial/joybus"
)

const pakSize = 1 << 16 // whole addressable space

const (
	blockSize = 32 // Number of bytes in a single read/write joybus command
	blockMask = blockSize - 1
)

const (
	pakLabel  = 0x0000
	pakProbe  = 0x8000 + 0x1f
	pakRumble = 0xC000 + 0x1f
)

// Values written to pakProbe to identify pak type. If the pak is capable of
// power on/off, writing the probe value also powers the pak on.
const (
	probeMem         = 0x01
	probeRumble      = 0x80
	probeBioSensor   = 0x81
	probeTransfer    = 0x84
	probeSnapStation = 0x85

	probePowerOff = 0xfe // Special value powers off the pak, if supported by the pak
)

// Pak represents a generic pak implementing [io.ReaderAt] and [io.WriterAt].
type Pak struct {
	port   uint8
	offset uint16

	readCmdBlock  serial.CommandBlock
	writeCmdBlock serial.CommandBlock
	readCmd       joybus.ReadPakCommand
	writeCmd      joybus.WritePakCommand
}

func newPak(port uint8) (pak *Pak) {
	pak = &Pak{
		port:          port,
		readCmdBlock:  *serial.NewCommandBlock(serial.CmdConfigureJoybus),
		writeCmdBlock: *serial.NewCommandBlock(serial.CmdConfigureJoybus),
	}

	var err error
	for range pak.port {
		err = joybus.ControlByte(&pak.readCmdBlock, joybus.CtrlSkip)
		debug.AssertErrNil(err)
	}
	pak.readCmd, err = joybus.NewReadPakCommand(&pak.readCmdBlock)
	err = joybus.ControlByte(&pak.readCmdBlock, joybus.CtrlAbort)
	debug.AssertErrNil(err)

	for range pak.port {
		err = joybus.ControlByte(&pak.writeCmdBlock, joybus.CtrlSkip)
		debug.AssertErrNil(err)
	}
	pak.writeCmd, err = joybus.NewWritePakCommand(&pak.writeCmdBlock)
	err = joybus.ControlByte(&pak.writeCmdBlock, joybus.CtrlAbort)
	debug.AssertErrNil(err)

	return
}

func (pak *Pak) ReadAt(p []byte, off int64) (n int, err error) {
	startOffset := off & blockMask

	for n < len(p) {
		pak.readCmd.Reset()
		pak.readCmd.SetAddress(uint16(off))
		serial.Run(&pak.readCmdBlock)

		var rx []byte
		rx, err = pak.readCmd.Data()
		copied := copy(p[n:], rx[startOffset:])
		n += copied
		startOffset = 0 // reset, only for first iteration needed
		if err != nil {
			return
		}

		off += int64(copied)
		if off >= pakSize {
			return n, io.EOF
		}
	}

	return
}

func (pak *Pak) WriteAt(p []byte, off int64) (n int, err error) {
	var tmp [blockSize]byte

	startOffset := off & blockMask

	for n < len(p) {
		// read first and last blocks if only partly written
		if startOffset != 0 || len(p[n:]) < blockSize {
			_, err = pak.ReadAt(tmp[:], off&^blockMask)
			if err != nil {
				return
			}
		}

		copied := copy(tmp[startOffset:], p[n:])
		startOffset = 0 // reset, only for first iteration needed

		pak.writeCmd.Reset()
		err = pak.writeCmd.SetData(tmp[:])
		if err != nil {
			return
		}

		pak.writeCmd.SetAddress(uint16(off))

		serial.Run(&pak.writeCmdBlock)

		err = pak.writeCmd.Result()
		if err != nil {
			return
		}

		n += copied

		off += int64(copied)
		if off >= pakSize {
			return n, io.EOF
		}
	}

	return
}

// ProbePak tries to identify the paks type and returns a [MemPak], [RumblePak]
// or [TransferPak] respectively. If no type could be determined, a generic
// [Pak] is returned.
func ProbePak(port uint8) (io.ReaderAt, error) {
	var err error
	pak := newPak(port)

	// Controller Pak is special as it does use pakProbe for SRAM bank
	// selection. Probe by looking for a filesystem instead.
	_, errFS := pakfs.Read(pak)
	if errFS == nil {
		return &MemPak{*pak}, nil
	}

	data := [1]byte{}
	types := [...]struct {
		probeVal byte
		ctor     func(*Pak) (io.ReaderAt, error)
	}{
		{probeMem, nil}, // controller pak with damaged filesystem
		{probeRumble, newRumblePak},
		{probeTransfer, newTransferPak},
	}

	for _, t := range types {
		data[0] = t.probeVal
		_, err = pak.WriteAt(data[:], pakProbe)
		if err != nil {
			return nil, err
		}

		_, err = pak.ReadAt(data[:], pakProbe)
		if err != nil {
			return nil, err
		}

		if data[0] == t.probeVal {
			if t.ctor == nil {
				break
			}
			return t.ctor(pak)
		}
	}

	// No type detected, return generic Pak.
	return pak, nil
}

// MemPak represents a Controller Pak with a [pakfs.FS] filesystem.
type MemPak struct {
	Pak
}

func newMemPak(pak *Pak) (io.ReaderAt, error) {
	return &MemPak{*pak}, nil
}

// RumblePak represents Rumble Pak providing force feedback.
type RumblePak struct {
	Pak
	on bool
}

func newRumblePak(pak *Pak) (io.ReaderAt, error) {
	return &RumblePak{*pak, false}, nil
}

// Set enables or disables vibration.
func (pak *RumblePak) Set(on bool) error {
	var data byte
	if on {
		data = 1
	}

	_, err := pak.WriteAt([]byte{data}, pakRumble)
	if err != nil {
		return err
	}
	pak.on = on

	return nil
}

// Toggle toggles vibration based on it's current state.
func (pak *RumblePak) Toggle() error {
	return pak.Set(!pak.on)
}

// TransferPak represents a Transfer Pak providing read and write access to an
// Game Boy cartridge.
type TransferPak struct {
	Pak
}

func newTransferPak(pak *Pak) (io.ReaderAt, error) {
	return &TransferPak{*pak}, nil
}
