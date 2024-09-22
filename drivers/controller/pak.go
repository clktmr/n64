package controller

import (
	"errors"
	"fmt"
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

// Values written to pakProbe to identify pak type.  If the pak is capable of
// power on/off, writing the probe value also powers the pak on.
const (
	probeMem         = 0x01
	probeRumble      = 0x80
	probeBioSensor   = 0x81
	probeTransfer    = 0x84
	probeSnapStation = 0x85

	probePowerOff = 0xfe // Special value powers off the pak, if supported by the pak
)

var ErrSeekOutOfRange = errors.New("pak seek out of range")

type Pak struct {
	port   uint8
	offset uint16

	readCmdBlock  serial.CommandBlock
	writeCmdBlock serial.CommandBlock
	readCmd       joybus.ReadPakCommand
	writeCmd      joybus.WritePakCommand
}

func NewPak(port uint8) (pak *Pak) {
	pak = &Pak{
		port:          port,
		readCmdBlock:  *serial.NewCommandBlock(serial.CmdConfigureJoybus),
		writeCmdBlock: *serial.NewCommandBlock(serial.CmdConfigureJoybus),
	}

	var err error
	for range pak.port {
		err = joybus.ControlByte(&pak.readCmdBlock, joybus.CtrlReset)
		debug.Assert(err == nil, fmt.Sprint(err))
	}
	pak.readCmd, err = joybus.NewReadPakCommand(&pak.readCmdBlock)
	err = joybus.ControlByte(&pak.readCmdBlock, joybus.CtrlAbort)
	debug.Assert(err == nil, fmt.Sprint(err))

	for range pak.port {
		err = joybus.ControlByte(&pak.writeCmdBlock, joybus.CtrlReset)
		debug.Assert(err == nil, fmt.Sprint(err))
	}
	pak.writeCmd, err = joybus.NewWritePakCommand(&pak.writeCmdBlock)
	err = joybus.ControlByte(&pak.writeCmdBlock, joybus.CtrlAbort)
	debug.Assert(err == nil, fmt.Sprint(err))

	return
}

func (pak *Pak) ReadAt(p []byte, off int64) (n int, err error) {
	startOffset := off & blockMask

	for n < len(p) {
		pak.readCmd.Reset() // TODO necessary?
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

func (pak *Pak) Read(p []byte) (n int, err error) {
	n, err = pak.ReadAt(p, int64(pak.offset))
	pak.offset += uint16(n)
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

		pak.writeCmd.Reset() // TODO necessary?
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

func (pak *Pak) Write(p []byte) (n int, err error) {
	n, err = pak.WriteAt(p, int64(pak.offset))
	pak.offset += uint16(n)
	return
}

func (pak *Pak) Seek(offset int64, whence int) (newoffset int64, err error) {
	switch whence {
	case io.SeekStart:
		// newoffset = 0
	case io.SeekCurrent:
		newoffset = int64(pak.offset)
	case io.SeekEnd:
		newoffset = pakSize
	}
	newoffset += offset
	if newoffset < 0 || newoffset > pakSize {
		return int64(pak.offset), fmt.Errorf("%w: %d", ErrSeekOutOfRange, newoffset)
	}

	pak.offset = uint16(newoffset)

	return
}

func ProbePak(port uint8) (io.ReadWriteSeeker, error) {
	var err error
	pak := NewPak(port)

	// Controller Pak is special as it does use pakProbe for SRAM bank
	// selection.  Probe by looking for a filesystem instead.
	_, errFS := pakfs.Read(pak)
	if errFS == nil {
		return &MemPak{*pak}, nil
	}

	data := [1]byte{}
	types := [...]struct {
		probeVal byte
		ctor     func(*Pak) (io.ReadWriteSeeker, error)
	}{
		{probeMem, nil}, // controller pak with damaged filesystem
		{probeRumble, newRumblePak},
		{probeTransfer, newTransferPak},
	}

	for _, t := range types {
		pak.Seek(pakProbe, io.SeekStart)
		data[0] = t.probeVal
		_, _ = pak.Write(data[:])
		if err != nil {
			return nil, err
		}

		pak.Seek(pakProbe, io.SeekStart)
		_, err = pak.Read(data[:])
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

type MemPak struct {
	Pak
}

func newMemPak(pak *Pak) (io.ReadWriteSeeker, error) {
	return &MemPak{*pak}, nil
}

type RumblePak struct {
	Pak
	on bool
}

func newRumblePak(pak *Pak) (io.ReadWriteSeeker, error) {
	return &RumblePak{*pak, false}, nil
}

func (pak *RumblePak) Set(on bool) error {
	var data byte
	if on {
		data = 1
	}

	pak.Seek(pakRumble, io.SeekStart)
	_, err := pak.Write([]byte{data})
	if err != nil {
		return err
	}
	pak.on = on

	return nil
}

func (pak *RumblePak) Toggle() error {
	return pak.Set(!pak.on)
}

type TransferPak struct {
	Pak
}

func newTransferPak(pak *Pak) (io.ReadWriteSeeker, error) {
	return &TransferPak{*pak}, nil
}
